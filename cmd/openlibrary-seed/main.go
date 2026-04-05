// openlibrary-seed 通过 Open Library 公开 JSON API（https://openlibrary.org/dev/docs/api/books）
// 按 ISBN 拉取书目元数据写入 MongoDB，避免手写简介、便于向量检索。
// 请控制请求频率（默认约 1 次/秒），勿对 openlibrary.org 并发压测。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/qiwang/book-e-commerce-micro/service/book/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	olBase       = "https://openlibrary.org"
	olUserAgent  = "BookHiveOpenLibrarySeed/1.0 (+https://github.com/qiwang/book-e-commerce-micro)"
	minRequestGap = 1100 * time.Millisecond
)

func main() {
	mode := flag.String("mode", "isbn", "运行模式：isbn（按 ISBN 列表）或 search（按主题词批量搜索）")
	mongoURI := flag.String("mongo-uri", "", "MongoDB URI（必填）")
	database := flag.String("database", "bookhive", "数据库名")
	delay := flag.Duration("delay", minRequestGap, "两次 Open Library 请求之间的间隔")

	// isbn 模式
	isbnsFile := flag.String("isbns-file", "", "[isbn] ISBN 列表文件")
	ifEmpty := flag.Bool("if-empty", false, "[isbn] 若 books 集合已有文档则直接退出 0")
	upsert := flag.Bool("upsert", false, "[isbn] 按 isbn upsert")
	dryRun := flag.Bool("dry-run", false, "[isbn] 只拉取并打印 JSON，不写库")

	// search 模式
	subjects := flag.String("subjects", "", "[search] 逗号分隔的主题词；空则用内置默认列表")
	perSubject := flag.Int("per-subject", 800, "[search] 每个主题最多拉多少条")
	total := flag.Int("total", 10000, "[search] 总上限")
	fillDesc := flag.Bool("fill-desc", false, "[search] 拉完后逐条补 description（慢，约 1 小时/万条）")

	flag.Parse()

	ctx := context.Background()
	hc := &http.Client{Timeout: 45 * time.Second}
	rl := newRateLimiter(*delay)

	switch *mode {
	case "search":
		if *mongoURI == "" {
			log.Fatal("-mongo-uri 必填")
		}
		coll := mustConnectMongo(ctx, *mongoURI, *database)
		fixTextIndex(ctx, coll)

		subjectList := defaultSubjects
		if *subjects != "" {
			subjectList = strings.Split(*subjects, ",")
			for i := range subjectList {
				subjectList[i] = strings.TrimSpace(subjectList[i])
			}
		}
		runSearchMode(ctx, hc, rl, coll, searchConfig{
			subjects:   subjectList,
			perSubject: *perSubject,
			total:      *total,
			fillDesc:   *fillDesc,
		})

	case "isbn":
		runISBNMode(ctx, hc, rl, *mongoURI, *database, *isbnsFile, *ifEmpty, *upsert, *dryRun)

	default:
		log.Fatalf("unknown mode: %s (use isbn or search)", *mode)
	}
}

func mustConnectMongo(ctx context.Context, uri, database string) *mongo.Collection {
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	if err := mc.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}
	return mc.Database(database).Collection("books")
}

func runISBNMode(ctx context.Context, hc *http.Client, rl *rateLimiter, mongoURI, database, isbnsFile string, ifEmpty, upsertFlag, dryRun bool) {
	if isbnsFile == "" {
		log.Fatal("-isbns-file 必填 (isbn 模式)")
	}
	raw, err := os.ReadFile(isbnsFile)
	if err != nil {
		log.Fatalf("read isbns file: %v", err)
	}
	isbns := parseISBNLines(string(raw))
	if len(isbns) == 0 {
		log.Fatal("ISBN 列表为空")
	}

	var books []model.Book
	for _, isbn := range isbns {
		b, err := fetchBook(ctx, hc, rl, isbn)
		if err != nil {
			log.Printf("skip isbn=%s: %v", isbn, err)
			continue
		}
		books = append(books, *b)
	}

	if len(books) == 0 {
		log.Fatal("没有成功拉取到任何书目")
	}

	if dryRun {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(books)
		return
	}

	if mongoURI == "" {
		log.Fatal("非 dry-run 时必须提供 -mongo-uri")
	}

	coll := mustConnectMongo(ctx, mongoURI, database)
	fixTextIndex(ctx, coll)

	n, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Fatalf("count books: %v", err)
	}
	if ifEmpty && n > 0 {
		log.Printf("openlibrary-seed: skip (--if-empty 且已有 %d 条)", n)
		return
	}

	if !ifEmpty && !upsertFlag && n > 0 {
		log.Fatal("books 非空：请使用 --if-empty 跳过，或 --upsert 按 ISBN 更新，或先清空集合")
	}

	if upsertFlag {
		for i := range books {
			b := &books[i]
			filter := bson.M{"isbn": b.ISBN}
			update := bson.M{"$set": b}
			opts := options.Update().SetUpsert(true)
			_, err := coll.UpdateOne(ctx, filter, update, opts)
			if err != nil {
				log.Fatalf("upsert isbn=%s: %v", b.ISBN, err)
			}
			log.Printf("upserted: %s — %s", b.ISBN, b.Title)
		}
		log.Printf("openlibrary-seed: done, upserted %d", len(books))
		return
	}

	docs := make([]interface{}, len(books))
	for i := range books {
		docs[i] = books[i]
	}
	_, err = coll.InsertMany(ctx, docs)
	if err != nil {
		log.Fatalf("insert many: %v", err)
	}
	log.Printf("openlibrary-seed: inserted %d books", len(books))
}

// fixTextIndex drops the legacy text index (default_language != "none") that rejects
// documents with language values like "zh-CN", then recreates it with language_override
// set to an unused field so the "language" field in book documents won't interfere.
func fixTextIndex(ctx context.Context, coll *mongo.Collection) {
	cur, err := coll.Indexes().List(ctx)
	if err != nil {
		log.Printf("fixTextIndex: list indexes: %v", err)
		return
	}
	defer cur.Close(ctx)

	var needsDrop string
	for cur.Next(ctx) {
		raw := cur.Current
		nameVal, _ := raw.Lookup("name").StringValueOK()
		defLang, _ := raw.Lookup("default_language").StringValueOK()
		langOverride, _ := raw.Lookup("language_override").StringValueOK()

		keyDoc, ok := raw.Lookup("key").DocumentOK()
		if !ok {
			continue
		}
		isText := false
		elems, _ := keyDoc.Elements()
		for _, e := range elems {
			if v, vok := e.Value().StringValueOK(); vok && v == "text" {
				isText = true
				break
			}
		}
		if !isText {
			continue
		}
		if defLang == "none" && langOverride == "_lang_unused" {
			return
		}
		needsDrop = nameVal
		break
	}

	if needsDrop == "" {
		log.Println("fixTextIndex: no legacy text index found, will create")
	} else {
		log.Printf("fixTextIndex: dropping legacy text index %q (default_language != none)", needsDrop)
		if _, err := coll.Indexes().DropOne(ctx, needsDrop); err != nil {
			log.Printf("fixTextIndex: drop error: %v", err)
			return
		}
	}

	idx := mongo.IndexModel{
		Keys: bson.D{
			{Key: "title", Value: "text"},
			{Key: "author", Value: "text"},
			{Key: "description", Value: "text"},
		},
		Options: options.Index().
			SetDefaultLanguage("none").
			SetLanguageOverride("_lang_unused"),
	}
	name, err := coll.Indexes().CreateOne(ctx, idx)
	if err != nil {
		log.Printf("fixTextIndex: create error: %v", err)
		return
	}
	log.Printf("fixTextIndex: created text index %q with default_language=none", name)
}

type rateLimiter struct {
	mu   sync.Mutex
	last time.Time
	gap  time.Duration
}

func newRateLimiter(gap time.Duration) *rateLimiter {
	if gap <= 0 {
		gap = minRequestGap
	}
	return &rateLimiter{gap: gap}
}

func (r *rateLimiter) wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.last.IsZero() {
		wait := r.gap - time.Since(r.last)
		if wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	r.last = time.Now()
	return nil
}

func parseISBNLines(s string) []string {
	seen := make(map[string]struct{})
	var out []string
	re := regexp.MustCompile(`[^0-9Xx]`)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		digits := re.ReplaceAllString(fields[0], "")
		if len(digits) < 10 {
			continue
		}
		digits = strings.ToUpper(digits)
		if _, ok := seen[digits]; ok {
			continue
		}
		seen[digits] = struct{}{}
		out = append(out, digits)
	}
	return out
}

func fetchBook(ctx context.Context, hc *http.Client, rl *rateLimiter, isbn string) (*model.Book, error) {
	editionPath := fmt.Sprintf("%s/isbn/%s.json", olBase, isbn)
	var ed editionDoc
	if err := getJSON(ctx, hc, rl, editionPath, &ed); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(ed.Title)
	title = strings.TrimPrefix(title, ": ")
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("edition 无 title")
	}

	var workKey string
	if len(ed.Works) > 0 {
		workKey = ed.Works[0].Key
	}

	var desc string
	var subjects []string
	var workAuthors []authorEntry
	if workKey != "" {
		var w workDoc
		if err := getJSON(ctx, hc, rl, olBase+workKey+".json", &w); err != nil {
			log.Printf("warn work %s: %v", workKey, err)
		} else {
			desc = normalizeDescription(w.Description)
			subjects = w.Subjects
			workAuthors = w.Authors
		}
	}

	authorSrc := ed.Authors
	if len(authorSrc) == 0 {
		authorSrc = workAuthors
	}
	var authorNames []string
	for _, a := range authorSrc {
		key := a.resolveAuthorKey()
		if key == "" {
			continue
		}
		var au authorDoc
		if err := getJSON(ctx, hc, rl, olBase+key+".json", &au); err != nil {
			log.Printf("warn author %s: %v", key, err)
			continue
		}
		name := strings.TrimSpace(au.Name)
		if name == "" {
			name = strings.TrimSpace(au.PersonalName)
		}
		if name != "" {
			authorNames = append(authorNames, name)
		}
	}
	author := strings.Join(authorNames, " / ")
	if author == "" {
		author = "Unknown"
	}

	publisher := ""
	if len(ed.Publishers) > 0 {
		publisher = ed.Publishers[0]
	}

	pages := int32(ed.NumberOfPages)
	if pages <= 0 && ed.NumberOfPagesMedian > 0 {
		pages = int32(ed.NumberOfPagesMedian)
	}

	lang := mapOLLanguage(ed.Languages)
	category, subcategory, tags := subjectsToCategories(subjects)

	now := time.Now()
	b := &model.Book{
		Title:       title,
		Author:      author,
		ISBN:        isbn,
		Publisher:   publisher,
		PublishDate: strings.TrimSpace(ed.PublishDate),
		Price:       0,
		Category:    category,
		Subcategory: subcategory,
		Description: desc,
		CoverURL:    fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg", isbn),
		Pages:       pages,
		Language:    lang,
		Tags:        tags,
		Rating:      0,
		RatingCount: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return b, nil
}

type editionDoc struct {
	Title               string         `json:"title"`
	Publishers          []string       `json:"publishers"`
	PublishDate         string         `json:"publish_date"`
	NumberOfPages       int            `json:"number_of_pages"`
	NumberOfPagesMedian int            `json:"number_of_pages_median"`
	Authors             []authorEntry  `json:"authors"`
	Works               []refKey       `json:"works"`
	Languages           []refKey       `json:"languages"`
}

// OL 常见两种：{"key":"/authors/OL1A"} 或 {"author":{"key":"/authors/OL1A"},"type":{...}}
type authorEntry struct {
	Key    string  `json:"key"`
	Author *refKey `json:"author"`
}

func (e authorEntry) resolveAuthorKey() string {
	if e.Key != "" {
		return e.Key
	}
	if e.Author != nil && e.Author.Key != "" {
		return e.Author.Key
	}
	return ""
}

type refKey struct {
	Key string `json:"key"`
}

type authorDoc struct {
	Name         string `json:"name"`
	PersonalName string `json:"personal_name"`
}

type workDoc struct {
	Description json.RawMessage `json:"description"`
	Subjects    []string        `json:"subjects"`
	Authors     []authorEntry   `json:"authors"`
}

func normalizeDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	var m struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &m) == nil {
		return strings.TrimSpace(m.Value)
	}
	return strings.TrimSpace(string(raw))
}

func mapOLLanguage(langs []refKey) string {
	if len(langs) == 0 {
		return "en"
	}
	k := langs[0].Key
	switch {
	case strings.Contains(k, "/eng"):
		return "en"
	case strings.Contains(k, "/chi"), strings.Contains(k, "/zho"):
		return "zh-CN"
	case strings.Contains(k, "/jpn"):
		return "ja"
	case strings.Contains(k, "/fre"), strings.Contains(k, "/fra"):
		return "fr"
	case strings.Contains(k, "/spa"):
		return "es"
	case strings.Contains(k, "/ger"), strings.Contains(k, "/deu"):
		return "de"
	default:
		return "en"
	}
}

func subjectsToCategories(subjects []string) (category, subcategory string, tags []string) {
	if len(subjects) == 0 {
		return "General", "", nil
	}
	category = trimSubject(subjects[0])
	if len(subjects) > 1 {
		subcategory = trimSubject(subjects[1])
	}
	for _, s := range subjects {
		t := trimSubject(s)
		if t == "" {
			continue
		}
		tags = append(tags, t)
		if len(tags) >= 12 {
			break
		}
	}
	return category, subcategory, tags
}

func trimSubject(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "/"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func getJSON(ctx context.Context, hc *http.Client, rl *rateLimiter, url string, v interface{}) error {
	if err := rl.wait(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", olUserAgent)
	req.Header.Set("Accept", "application/json")

	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s: %s", res.Status, truncate(string(body), 200))
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
