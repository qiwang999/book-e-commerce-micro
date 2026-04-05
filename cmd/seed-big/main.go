// Command seed-big loads large-scale demo data: MongoDB books + MySQL main + order/inventory shards.
//
// Default MongoDB URI has no credentials (typical local install: "Access control is not enabled").
// If you use compose Mongo with MONGO_INITDB_ROOT_*, pass e.g.:
//
//	-mongo-uri='mongodb://bookhive:bookhive123@127.0.0.1:27017/?authSource=admin&authMechanism=SCRAM-SHA-256'
//
// Docker MySQL ports on host (example):
//
//	go run ./cmd/seed-big -truncate -mongo-wipe \
//	  -users=1000000 -orders=800000 -inventory-rows=1200000 -payments=400000 -books=50000
//
// Quick smoke test:
//
//	go run ./cmd/seed-big -truncate -mongo-wipe -users=20000 -orders=5000 -inventory-rows=30000 -payments=2000 -books=2000
//
// 通过 Book gRPC CreateBook 灌书（会发 RabbitMQ book.changed，AI 可增量 EmbedBook；需 Consul + book-service 已启动）:
//
//	go run ./cmd/seed-big -books-use-grpc -consul-addr=127.0.0.1:8500 -mongo-wipe -books=500 ...
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-micro/plugins/v4/registry/consul"
	_ "github.com/go-sql-driver/mysql"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/qiwang/book-e-commerce-micro/common"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	"go-micro.dev/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

const numOrderShards = 4
const numInvShards = 4

func main() {
	var (
		truncate  = flag.Bool("truncate", false, "TRUNCATE MySQL tables before insert (destructive)")
		mongoWipe = flag.Bool("mongo-wipe", false, "drop MongoDB books collection before insert")

		// Default: no auth (local Mongo). Compose Mongo with root user: pass bookhive@...?authSource=admin&authMechanism=SCRAM-SHA-256
		mongoURI = flag.String("mongo-uri", "mongodb://127.0.0.1:27017", "MongoDB URI")
		mongoDB    = flag.String("mongo-db", "bookhive", "MongoDB database name")
		booksCount = flag.Int("books", 50000, "books in MongoDB (0 = skip Mongo, use synthetic hex book_ids)")

		booksUseGRPC = flag.Bool("books-use-grpc", false, "create each book via BookService.CreateBook (triggers book.changed → AI embedding); slower, needs Consul + book-service + RabbitMQ on book side")
		consulAddr   = flag.String("consul-addr", "127.0.0.1:8500", "Consul address for go-micro (used when -books-use-grpc)")

		mainDSN = flag.String("main-dsn", "root:bookhive123@tcp(127.0.0.1:3306)/bookhive?parseTime=true&multiStatements=true", "Main MySQL DSN")

		orderDSNs = flag.String("order-dsns", "root:bookhive123@tcp(127.0.0.1:3307)/bookhive_order_0,root:bookhive123@tcp(127.0.0.1:3307)/bookhive_order_1,root:bookhive123@tcp(127.0.0.1:3308)/bookhive_order_2,root:bookhive123@tcp(127.0.0.1:3308)/bookhive_order_3", "four order shard DSNs, comma-separated")
		invDSNs   = flag.String("inv-dsns", "root:bookhive123@tcp(127.0.0.1:3309)/bookhive_inventory_0,root:bookhive123@tcp(127.0.0.1:3309)/bookhive_inventory_1,root:bookhive123@tcp(127.0.0.1:3310)/bookhive_inventory_2,root:bookhive123@tcp(127.0.0.1:3310)/bookhive_inventory_3", "four inventory shard DSNs")

		usersN    = flag.Int("users", 1_000_000, "users and user_profiles")
		storesN   = flag.Int("stores", 400, "store rows (POINT in East Asia bbox)")
		ordersN   = flag.Int("orders", 800_000, "orders across shards")
		itemsPer  = flag.Int("items-per-order", 1, "order_items per order")
		invRows   = flag.Int("inventory-rows", 1_200_000, "store_inventory rows total")
		paymentsN = flag.Int("payments", 400_000, "payments on main DB (demo refs; order_id not globally unique across shards)")
		batchSize = flag.Int("batch", 3000, "INSERT batch size")
		seedArg   = flag.Int64("seed", 42, "RNG seed")
	)
	flag.Parse()

	rand.Seed(*seedArg + time.Now().UnixNano())

	if *usersN < 1 || *storesN < 4 {
		log.Fatal("users >= 1 and stores >= 4 required")
	}
	if *ordersN < 0 || *invRows < 0 || *paymentsN < 0 {
		log.Fatal("counts must be non-negative")
	}

	orderShards := splitDSNs(*orderDSNs)
	invShards := splitDSNs(*invDSNs)
	if len(orderShards) != numOrderShards || len(invShards) != numInvShards {
		log.Fatalf("need exactly %d order DSNs and %d inventory DSNs", numOrderShards, numInvShards)
	}

	passHash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		log.Fatal(err)
	}
	passHashStr := string(passHash)

	mainDB, err := sql.Open("mysql", *mainDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer mainDB.Close()
	mainDB.SetMaxOpenConns(4)
	if err := mainDB.Ping(); err != nil {
		log.Fatalf("main mysql ping: %v", err)
	}

	orderDBs := openAll(orderShards)
	defer closeAll(orderDBs)
	invDBs := openAll(invShards)
	defer closeAll(invDBs)

	ctx := context.Background()

	var bookIDs []string
	if *booksCount > 0 {
		var err error
		if *booksUseGRPC {
			log.Printf("Books: creating %d via Book gRPC (book.changed)…", *booksCount)
			bookCli, err := newBookClient(*consulAddr)
			if err != nil {
				log.Fatalf("book grpc client: %v", err)
			}
			bookIDs, err = seedBooksViaGRPC(ctx, bookCli, *booksCount, *mongoWipe, *mongoURI, *mongoDB)
			if err != nil {
				log.Fatalf("book grpc seed: %v", err)
			}
			log.Printf("Books: gRPC done, %d book_ids (ensure AI service is consuming ai.book.embedding)", len(bookIDs))
		} else {
			log.Printf("MongoDB: inserting %d books (direct)…", *booksCount)
			bookIDs, err = seedMongoBooks(ctx, *mongoURI, *mongoDB, *booksCount, *batchSize, *mongoWipe)
			if err != nil {
				log.Fatalf("mongo: %v", err)
			}
			log.Printf("MongoDB: done, %d book_ids", len(bookIDs))
		}
	} else {
		log.Println("Skipping MongoDB; synthetic book_ids")
		for i := 0; i < 65536; i++ {
			bookIDs = append(bookIDs, fmt.Sprintf("%024x", uint64(i+1)))
		}
	}

	if *truncate {
		log.Println("MySQL: truncating...")
		if err := truncateAll(mainDB, orderDBs, invDBs); err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("stores: %d", *storesN)
	if err := seedStores(mainDB, *storesN, *batchSize); err != nil {
		log.Fatal(err)
	}

	log.Printf("users + profiles: %d", *usersN)
	if err := seedUsers(mainDB, *usersN, passHashStr, *batchSize); err != nil {
		log.Fatal(err)
	}

	log.Printf("orders: %d (items/order=%d)", *ordersN, *itemsPer)
	if err := seedOrders(orderDBs, *usersN, *storesN, *ordersN, *itemsPer, bookIDs, *batchSize); err != nil {
		log.Fatal(err)
	}

	log.Printf("inventory: %d rows", *invRows)
	if err := seedInventory(invDBs, *storesN, *invRows, bookIDs, *batchSize); err != nil {
		log.Fatal(err)
	}

	log.Printf("payments: %d", *paymentsN)
	if err := seedPayments(mainDB, *usersN, *ordersN, *paymentsN, *batchSize); err != nil {
		log.Fatal(err)
	}

	log.Println("All done.")
}

func splitDSNs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func openAll(dsns []string) []*sql.DB {
	out := make([]*sql.DB, len(dsns))
	for i, dsn := range dsns {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatal(err)
		}
		db.SetMaxOpenConns(2)
		if err := db.Ping(); err != nil {
			log.Fatalf("ping %s: %v", dsn, err)
		}
		out[i] = db
	}
	return out
}

func closeAll(dbs []*sql.DB) {
	for _, db := range dbs {
		_ = db.Close()
	}
}

func truncateAll(main *sql.DB, orderDBs, invDBs []*sql.DB) error {
	tx, err := main.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`SET FOREIGN_KEY_CHECKS=0`); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, s := range []string{
		`TRUNCATE TABLE payments`,
		`TRUNCATE TABLE user_addresses`,
		`TRUNCATE TABLE user_profiles`,
		`TRUNCATE TABLE users`,
		`TRUNCATE TABLE inventory_locks`,
		`TRUNCATE TABLE order_items`,
		`TRUNCATE TABLE orders`,
		`TRUNCATE TABLE store_inventory`,
		`TRUNCATE TABLE stores`,
	} {
		if _, err := tx.Exec(s); err != nil {
			log.Printf("truncate (ok if missing): %s: %v", s, err)
		}
	}
	if _, err := tx.Exec(`SET FOREIGN_KEY_CHECKS=1`); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	for i, db := range orderDBs {
		for _, q := range []string{
			`SET FOREIGN_KEY_CHECKS=0`,
			`TRUNCATE TABLE order_items`,
			`TRUNCATE TABLE orders`,
			`SET FOREIGN_KEY_CHECKS=1`,
		} {
			if _, err := db.Exec(q); err != nil {
				return fmt.Errorf("order shard %d %s: %w", i, q, err)
			}
		}
	}
	for i, db := range invDBs {
		for _, q := range []string{
			`SET FOREIGN_KEY_CHECKS=0`,
			`TRUNCATE TABLE inventory_locks`,
			`TRUNCATE TABLE store_inventory`,
			`SET FOREIGN_KEY_CHECKS=1`,
		} {
			if _, err := db.Exec(q); err != nil {
				return fmt.Errorf("inv shard %d %s: %w", i, q, err)
			}
		}
	}
	return nil
}

// fixMongoURI adds authSource=admin and authMechanism=SCRAM-SHA-256 when the URI has credentials
// but omits them — fixes Docker MONGO_INITDB_ROOT_* + MongoDB 7 (SHA-256) handshake failures.
func fixMongoURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.User == nil || u.User.Username() == "" {
		return uri
	}
	q := u.Query()
	changed := false
	if q.Get("authSource") == "" {
		q.Set("authSource", "admin")
		changed = true
	}
	if q.Get("authMechanism") == "" {
		q.Set("authMechanism", "SCRAM-SHA-256")
		changed = true
	}
	if !changed {
		return uri
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func logMongoAuthHint() {
	log.Print("Mongo 认证失败时常见原因：① 数据卷是旧的，MONGO_INITDB_ROOT 只在「首次」建库生效 — 需删掉 mongo 数据卷后重新 up（会清空 Mongo）；② 27017 不是 compose 里的 mongo（本机另一个实例）— 请核对 `podman ps`/`docker ps`；③ 本机 Mongo 未开认证 — 使用: -mongo-uri=mongodb://127.0.0.1:27017")
}

func isMongoNamespaceNotFound(err error) bool {
	var ce mongo.CommandError
	return errors.As(err, &ce) && ce.Code == 26 // NamespaceNotFound
}

func dropMongoBooksCollection(ctx context.Context, uri, dbName string) error {
	uri = fixMongoURI(uri)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(30*time.Second))
	if err != nil {
		logMongoAuthHint()
		return err
	}
	defer client.Disconnect(ctx)
	coll := client.Database(dbName).Collection("books")
	if err := coll.Drop(ctx); err != nil && !isMongoNamespaceNotFound(err) {
		logMongoAuthHint()
		return fmt.Errorf("drop books: %w", err)
	}
	return nil
}

func newBookClient(consulAddress string) (bookPb.BookService, error) {
	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: consulAddress}))
	svc := micro.NewService(
		micro.Name("bookhive.seed-big"),
		micro.Version("1.0.0"),
		micro.Registry(reg),
	)
	origArgs := os.Args
	if len(origArgs) > 0 {
		os.Args = []string{origArgs[0]}
	}
	svc.Init()
	os.Args = origArgs
	return bookPb.NewBookService(common.ServiceBook, svc.Client()), nil
}

func seedBooksViaGRPC(ctx context.Context, bookCli bookPb.BookService, total int, wipe bool, mongoURI, dbName string) ([]string, error) {
	if wipe {
		log.Println("MongoDB: dropping books collection before gRPC seed (-mongo-wipe)")
		if err := dropMongoBooksCollection(ctx, mongoURI, dbName); err != nil {
			return nil, err
		}
	}
	cats := []string{"文学", "计算机", "科幻", "历史", "艺术", "童书", "经济", "生活"}
	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		req := &bookPb.CreateBookRequest{
			Title:       fmt.Sprintf("Seed Book %09d", i+1),
			Author:      fmt.Sprintf("Author %d", i%9999),
			Isbn:        fmt.Sprintf("978-%010d", i),
			Publisher:   "Seed Press",
			PublishDate: "2020-01-01",
			Price:       29.9 + float64(i%200),
			Category:    cats[i%len(cats)],
			Subcategory: "",
			Description: strings.Repeat("测", 80),
			CoverUrl:    "",
			Pages:       int32(200 + i%400),
			Language:    "zh",
			Tags:        []string{"seed"},
		}
		rsp, err := bookCli.CreateBook(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("CreateBook #%d: %w", i+1, err)
		}
		if rsp.GetId() == "" {
			return nil, fmt.Errorf("CreateBook #%d: empty id", i+1)
		}
		ids = append(ids, rsp.Id)
		if (i+1)%500 == 0 || i+1 == total {
			log.Printf("  book gRPC %d / %d", i+1, total)
		}
	}
	log.Printf("Book gRPC: 已创建 %d 本，Mongo 库 %q 集合 books；每本已通过 Book 服务写库并发 book.changed", len(ids), dbName)
	return ids, nil
}

func seedMongoBooks(ctx context.Context, uri, dbName string, total, batch int, wipe bool) ([]string, error) {
	uri = fixMongoURI(uri)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(30*time.Second))
	if err != nil {
		logMongoAuthHint()
		return nil, err
	}
	defer client.Disconnect(ctx)

	coll := client.Database(dbName).Collection("books")
	if wipe {
		if err := coll.Drop(ctx); err != nil && !isMongoNamespaceNotFound(err) {
			logMongoAuthHint()
			return nil, fmt.Errorf("drop books: %w", err)
		}
		coll = client.Database(dbName).Collection("books")
	}

	var ids []string
	now := time.Now()
	cats := []string{"文学", "计算机", "科幻", "历史", "艺术", "童书", "经济", "生活"}
	opts := options.InsertMany().SetOrdered(false)

	for start := 0; start < total; start += batch {
		end := start + batch
		if end > total {
			end = total
		}
		docs := make([]interface{}, 0, end-start)
		for i := start; i < end; i++ {
			oid := primitive.NewObjectID()
			title := fmt.Sprintf("Seed Book %09d", i+1)
			docs = append(docs, bson.M{
				"_id":           oid,
				"title":         title,
				"author":        fmt.Sprintf("Author %d", i%9999),
				"isbn":          fmt.Sprintf("978-%010d", i),
				"publisher":     "Seed Press",
				"publish_date":  "2020-01-01",
				"price":         29.9 + float64(i%200),
				"category":      cats[i%len(cats)],
				"subcategory":   "",
				"description":   strings.Repeat("测", 80),
				"cover_url":     "",
				"pages":         int32(200 + i%400),
				"language":      "zh",
				"tags":          []string{"seed"},
				"rating":        4.0,
				"rating_count":  int64(1 + i%50),
				"created_at":    now,
				"updated_at":    now,
			})
			ids = append(ids, oid.Hex())
		}
		if _, err := coll.InsertMany(ctx, docs, opts); err != nil {
			return nil, err
		}
		if end%(batch*10) == 0 || end == total {
			log.Printf("  mongo %d / %d", end, total)
		}
	}

	n, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Printf("MongoDB: count documents: %v", err)
	} else {
		log.Printf("MongoDB: 数据在库 %q、集合 books，当前文档数=%d。mongosh 里请先: use bookhive  再: db.books.countDocuments()", dbName, n)
	}
	return ids, nil
}

func seedStores(db *sql.DB, n, batch int) error {
	const q = `INSERT INTO stores (name, description, address, city, district, phone, location, business_hours) VALUES `
	for start := 0; start < n; start += batch {
		end := start + batch
		if end > n {
			end = n
		}
		var b strings.Builder
		b.WriteString(q)
		first := true
		for i := start; i < end; i++ {
			id := i + 1
			lat := 25.0 + rand.Float64()*18.0
			lon := 100.0 + rand.Float64()*25.0
			if !first {
				b.WriteByte(',')
			}
			first = false
			fmt.Fprintf(&b, "(%s,%s,%s,%s,%s,%s,ST_GeomFromText('POINT(%f %f)', 4326),%s)",
				esc(fmt.Sprintf("Seed Store %d", id)),
				esc("bulk seed"),
				esc(fmt.Sprintf("Addr %d", id)),
				esc("SeedCity"),
				esc("Dist"),
				esc("01000000000"),
				lat, lon,
				esc("09:00-21:00"))
		}
		if _, err := db.Exec(b.String()); err != nil {
			return fmt.Errorf("stores %d-%d: %w", start, end, err)
		}
		log.Printf("  stores %d / %d", end, n)
	}
	return nil
}

func esc(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func seedUsers(db *sql.DB, n int, passHash string, batch int) error {
	for start := 0; start < n; start += batch {
		end := start + batch
		if end > n {
			end = n
		}
		var ub strings.Builder
		ub.WriteString(`INSERT INTO users (email, password_hash, name, role, status) VALUES `)
		first := true
		for i := start; i < end; i++ {
			uid := i + 1
			if !first {
				ub.WriteByte(',')
			}
			first = false
			fmt.Fprintf(&ub, "(%s,%s,%s,'customer',1)",
				esc(fmt.Sprintf("user_%d@seed.local", uid)),
				esc(passHash),
				esc(fmt.Sprintf("U%d", uid)))
		}
		if _, err := db.Exec(ub.String()); err != nil {
			return fmt.Errorf("users %d-%d: %w", start, end, err)
		}

		var pb strings.Builder
		pb.WriteString(`INSERT INTO user_profiles (user_id, phone, gender) VALUES `)
		first = true
		for i := start; i < end; i++ {
			uid := i + 1
			if !first {
				pb.WriteByte(',')
			}
			first = false
			fmt.Fprintf(&pb, "(%d,'',0)", uid)
		}
		if _, err := db.Exec(pb.String()); err != nil {
			return fmt.Errorf("profiles %d-%d: %w", start, end, err)
		}

		if end%(batch*5) == 0 || end == n {
			log.Printf("  users %d / %d", end, n)
		}
	}
	return nil
}

type orderRow struct {
	orderNo   string
	userID    uint64
	storeID   uint64
	total     float64
	status    string
	pickup    string
	items     []orderItemRow
}

type orderItemRow struct {
	bookID    string
	title     string
	author    string
	price     float64
	qty       int
}

func generateOrderNo(userID uint64, seq int) string {
	sk := userID % 100
	t := time.Now().UTC().Add(-time.Duration(seq%360000) * time.Second)
	return fmt.Sprintf("BH%02d%s%06d%05d", sk, t.Format("20060102150405"), (t.Nanosecond()/1000)%1000000, seq%100000)
}

func userIDForShard(shard, k, usersN int) uint64 {
	var uid uint64
	switch shard {
	case 0:
		uid = uint64(4 * (k + 1))
	default:
		uid = uint64(shard) + uint64(4*k)
	}
	if uid > uint64(usersN) {
		uid = (uid-1)%uint64(usersN) + 1
	}
	return uid
}

func seedOrders(dbs []*sql.DB, usersN, storesN, ordersN, itemsPer int, bookIDs []string, batch int) error {
	if len(bookIDs) == 0 {
		return fmt.Errorf("no book ids")
	}
	statuses := []string{"completed", "paid", "pending_payment", "cancelled"}
	pickups := []string{"in_store", "delivery"}

	buf := make([][]orderRow, numOrderShards)
	counts := make([]int, numOrderShards)
	target := make([]int, numOrderShards)
	base := ordersN / numOrderShards
	rem := ordersN % numOrderShards
	for s := 0; s < numOrderShards; s++ {
		target[s] = base
		if s < rem {
			target[s]++
		}
	}

	seq := 0
	for sh := 0; sh < numOrderShards; sh++ {
		for k := 0; k < target[sh]; k++ {
			seq++
			userID := userIDForShard(sh, k, usersN)
			storeID := uint64(1 + rand.Intn(storesN))
			st := statuses[rand.Intn(len(statuses))]
			pu := pickups[rand.Intn(len(pickups))]
			or := orderRow{
				orderNo: generateOrderNo(userID, seq),
				userID:  userID,
				storeID: storeID,
				total:   19.9 + float64(rand.Intn(500)),
				status:  st,
				pickup:  pu,
			}
			for it := 0; it < itemsPer; it++ {
				bid := bookIDs[(seq+it)%len(bookIDs)]
				prefix := bid
				if len(prefix) > 8 {
					prefix = prefix[:8]
				}
				or.items = append(or.items, orderItemRow{
					bookID: bid,
					title:  fmt.Sprintf("Book %s", prefix),
					author: "Seed",
					price:  29.9,
					qty:    1 + rand.Intn(3),
				})
			}
			buf[sh] = append(buf[sh], or)
			counts[sh]++
			if len(buf[sh]) >= batch {
				if err := flushOrders(dbs[sh], buf[sh]); err != nil {
					return err
				}
				buf[sh] = buf[sh][:0]
			}
		}
		if len(buf[sh]) > 0 {
			if err := flushOrders(dbs[sh], buf[sh]); err != nil {
				return err
			}
			buf[sh] = buf[sh][:0]
		}
		log.Printf("  order shard %d: %d orders", sh, counts[sh])
	}
	return nil
}

func flushOrders(db *sql.DB, rows []orderRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var ob strings.Builder
	ob.WriteString(`INSERT INTO orders (order_no, user_id, store_id, total_amount, status, pickup_method) VALUES `)
	for i, r := range rows {
		if i > 0 {
			ob.WriteByte(',')
		}
		fmt.Fprintf(&ob, "(%s,%d,%d,%.2f,%s,%s)",
			esc(r.orderNo), r.userID, r.storeID, r.total, esc(r.status), esc(r.pickup))
	}
	res, err := tx.Exec(ob.String())
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	firstID, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	var ib strings.Builder
	ib.WriteString(`INSERT INTO order_items (order_id, book_id, book_title, book_author, book_cover, price, quantity) VALUES `)
	first := true
	for i, r := range rows {
		oid := uint64(firstID) + uint64(i)
		for _, it := range r.items {
			if !first {
				ib.WriteByte(',')
			}
			first = false
			fmt.Fprintf(&ib, "(%d,%s,%s,%s,'',%.2f,%d)",
				oid, esc(it.bookID), esc(it.title), esc(it.author), it.price, it.qty)
		}
	}
	if _, err := tx.Exec(ib.String()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func seedInventory(dbs []*sql.DB, storesN, totalRows int, bookIDs []string, batch int) error {
	if len(bookIDs) == 0 {
		return fmt.Errorf("no book ids")
	}
	buffers := make([]strings.Builder, numInvShards)
	counts := make([]int, numInvShards)
	shardRows := make([]int, numInvShards)

	flushShard := func(sh int) error {
		if counts[sh] == 0 {
			return nil
		}
		q := `INSERT INTO store_inventory (store_id, book_id, quantity, locked_quantity, price) VALUES ` + buffers[sh].String()
		if _, err := dbs[sh].Exec(q); err != nil {
			return fmt.Errorf("shard %d: %w", sh, err)
		}
		buffers[sh].Reset()
		counts[sh] = 0
		return nil
	}

	for row := 0; row < totalRows; row++ {
		storeID := 1 + (row % storesN)
		sh := storeID % numInvShards
		bookIdx := (row / storesN) % len(bookIDs)
		bid := bookIDs[bookIdx]
		qty := 10 + rand.Intn(200)
		price := 20.0 + float64(rand.Intn(80))
		if counts[sh] > 0 {
			buffers[sh].WriteByte(',')
		}
		fmt.Fprintf(&buffers[sh], "(%d,%s,%d,0,%.2f)", storeID, esc(bid), qty, price)
		counts[sh]++
		shardRows[sh]++
		if counts[sh] >= batch {
			if err := flushShard(sh); err != nil {
				return err
			}
		}
		if (row+1)%(batch*numInvShards*3) == 0 {
			log.Printf("  inventory progress %d / %d", row+1, totalRows)
		}
	}
	for sh := 0; sh < numInvShards; sh++ {
		if err := flushShard(sh); err != nil {
			return err
		}
		log.Printf("  inventory shard %d: %d rows", sh, shardRows[sh])
	}
	return nil
}

func seedPayments(db *sql.DB, usersN, ordersN, n, batch int) error {
	st := []string{"success", "pending", "failed", "processing"}
	for start := 0; start < n; start += batch {
		end := start + batch
		if end > n {
			end = n
		}
		var b strings.Builder
		b.WriteString(`INSERT INTO payments (payment_no, order_id, user_id, amount, method, status) VALUES `)
		for i := start; i < end; i++ {
			if i > start {
				b.WriteByte(',')
			}
			uid := uint64(1 + rand.Intn(usersN))
			oid := uint64(1 + rand.Intn(intMax(1, ordersN*2)))
			pn := fmt.Sprintf("PAY%015d%05d", time.Now().UnixNano()%1e15, i%100000)
			fmt.Fprintf(&b, "(%s,%d,%d,%.2f,'simulated',%s)",
				esc(pn), oid, uid, 10+float64(rand.Intn(1000)), esc(st[rand.Intn(len(st))]))
		}
		if _, err := db.Exec(b.String()); err != nil {
			return fmt.Errorf("payments %d-%d: %w", start, end, err)
		}
		if end%(batch*10) == 0 || end == n {
			log.Printf("  payments %d / %d", end, n)
		}
	}
	return nil
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
