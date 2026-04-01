package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-micro/plugins/v4/registry/consul"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/redis/go-redis/v9"
	"go-micro.dev/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/qiwang/book-e-commerce-micro/common"
	"github.com/qiwang/book-e-commerce-micro/common/config"
	pb "github.com/qiwang/book-e-commerce-micro/proto/ai"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
	cartPb "github.com/qiwang/book-e-commerce-micro/proto/cart"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
	paymentPb "github.com/qiwang/book-e-commerce-micro/proto/payment"
	"github.com/qiwang/book-e-commerce-micro/service/ai/agent"
	"github.com/qiwang/book-e-commerce-micro/service/ai/embedding"
	"github.com/qiwang/book-e-commerce-micro/service/ai/handler"
	"github.com/qiwang/book-e-commerce-micro/service/ai/rag"
	"github.com/qiwang/book-e-commerce-micro/service/ai/repository"
	aitools "github.com/qiwang/book-e-commerce-micro/service/ai/tools"
	"github.com/qiwang/book-e-commerce-micro/service/ai/vectorstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	// MongoDB
	mongoCtx, mongoCancel := context.WithTimeout(ctx, 10*time.Second)
	defer mongoCancel()

	mongoClient, err := mongo.Connect(mongoCtx, options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("failed to disconnect MongoDB: %v", err)
		}
	}()

	if err := mongoClient.Ping(mongoCtx, nil); err != nil {
		log.Fatalf("failed to ping MongoDB: %v", err)
	}
	log.Println("connected to MongoDB")

	db := mongoClient.Database(cfg.MongoDB.Database)

	// Redis (reserved for session caching)
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: Redis ping failed: %v (caching disabled)", err)
	} else {
		log.Println("connected to Redis")
	}

	repo := repository.NewAIRepository(db)

	// Milvus（专业向量库）；compose 使用官方多架构镜像 v2.5.12（含 linux/arm64），避免 ARM Mac 上 amd64+QEMU 不稳定
	milvusAddr := cfg.Milvus.Address
	if milvusAddr == "" {
		milvusAddr = "127.0.0.1:19530"
	}
	milvusStore, err := vectorstore.NewMilvusStore(ctx, milvusAddr)
	if err != nil {
		log.Fatalf("failed to connect to Milvus: %v", err)
	}
	defer milvusStore.Close()
	var store vectorstore.Store = milvusStore
	log.Println("connected to Milvus vector database")

	// go-micro service
	reg := consul.NewRegistry(consul.Config(&consulapi.Config{Address: cfg.Consul.Address}))

	svc := micro.NewService(
		micro.Name(common.ServiceAI),
		micro.Version("2.0.0"),
		micro.Address(fmt.Sprintf(":%d", cfg.Services.AI.GRPCPort)),
		micro.Registry(reg),
	)
	svc.Init()

	// gRPC clients for tools and user data
	bookClient := bookPb.NewBookService(common.ServiceBook, svc.Client())
	inventoryClient := inventoryPb.NewInventoryService(common.ServiceInventory, svc.Client())
	orderClient := orderPb.NewOrderService(common.ServiceOrder, svc.Client())
	cartClient := cartPb.NewCartService(common.ServiceCart, svc.Client())
	paymentClient := paymentPb.NewPaymentService(common.ServicePayment, svc.Client())

	hasAPIKey := cfg.OpenAI.APIKey != ""

	// Build Eino tools
	searchBooksTool, err := aitools.NewSearchBooksTool(bookClient)
	if err != nil {
		log.Fatalf("failed to create search_books tool: %v", err)
	}
	getBookDetailTool, err := aitools.NewGetBookDetailTool(bookClient)
	if err != nil {
		log.Fatalf("failed to create get_book_detail tool: %v", err)
	}
	checkStockTool, err := aitools.NewCheckStockTool(inventoryClient)
	if err != nil {
		log.Fatalf("failed to create check_stock tool: %v", err)
	}

	// Build Eino agents (only if API key is configured)
	var h *handler.AIHandler
	if hasAPIKey {
		// DeepSeek recommended temperatures:
		// - Librarian (1.3): 通用对话
		// - Recommender / Summarizer / SmartSearch / TasteAnalyzer (1.0): 数据抽取/分析
		// - IntentRouter (1.0): 数据抽取/分类
		chatModelConversation, err := agent.NewChatModelWithTemperature(ctx, &cfg.OpenAI, 1.3)
		if err != nil {
			log.Fatalf("failed to create conversation chatmodel: %v", err)
		}
		chatModelAnalysis, err := agent.NewChatModelWithTemperature(ctx, &cfg.OpenAI, 1.0)
		if err != nil {
			log.Fatalf("failed to create analysis chatmodel: %v", err)
		}
		log.Println("Eino ChatModels initialized (conversation=1.3, analysis=1.0)")

		// Embedding service (Eino embedder + vector store)
		embSvc, err := embedding.NewService(ctx, &cfg.OpenAI, store, bookClient)
		if err != nil {
			log.Fatalf("failed to create embedding service: %v", err)
		}
		log.Println("Eino Embedding service initialized (Milvus backend)")

		findSimilarTool, err := aitools.NewFindSimilarBooksTool(embSvc)
		if err != nil {
			log.Fatalf("failed to create find_similar_books tool: %v", err)
		}

		getUserOrdersTool, err := aitools.NewGetUserOrdersTool(orderClient)
		if err != nil {
			log.Fatalf("failed to create get_user_orders tool: %v", err)
		}

		addToCartTool, err := aitools.NewAddToCartTool(cartClient)
		if err != nil {
			log.Fatalf("failed to create add_to_cart tool: %v", err)
		}

		getCartTool, err := aitools.NewGetCartTool(cartClient)
		if err != nil {
			log.Fatalf("failed to create get_cart tool: %v", err)
		}

		createOrderTool, err := aitools.NewCreateOrderTool(orderClient)
		if err != nil {
			log.Fatalf("failed to create create_order tool: %v", err)
		}

		getOrderDetailTool, err := aitools.NewGetOrderDetailTool(orderClient)
		if err != nil {
			log.Fatalf("failed to create get_order_detail tool: %v", err)
		}

		cancelOrderTool, err := aitools.NewCancelOrderTool(orderClient)
		if err != nil {
			log.Fatalf("failed to create cancel_order tool: %v", err)
		}

		createPaymentTool, err := aitools.NewCreatePaymentTool(paymentClient)
		if err != nil {
			log.Fatalf("failed to create create_payment tool: %v", err)
		}

		// Tool Registry: register groups then tools
		registry := agent.NewToolRegistry()

		registry.RegisterGroup(agent.GroupDiscovery, agent.GroupMeta{
			Title:      "搜索与发现",
			Footer:     "善用搜索工具！先搜索再回答，避免凭空回答用户关于书的问题。",
			IntentHint: "搜索、推荐、找书、书籍详情、库存查询",
		})
		registry.RegisterGroup(agent.GroupShopping, agent.GroupMeta{
			Title:      "购物",
			Footer:     "加购前先确认用户的购买意图。",
			IntentHint: "加购物车、购买、选购",
			Keywords:   []string{"购物车", "加购", "cart", "add to cart", "买", "购买", "加入", "放入", "选购"},
		})
		registry.RegisterGroup(agent.GroupOrder, agent.GroupMeta{
			Title:      "下单与支付（敏感操作 — 必须确认）",
			Footer:     "必须遵循确认流程：展示摘要 → 等用户确认 → 才能执行。绝不自作主张。",
			IntentHint: "下单、结账、支付、付款",
			Keywords:   []string{"下单", "结账", "支付", "付款", "order", "pay", "checkout", "确认下单", "去支付", "微信", "支付宝"},
		})
		registry.RegisterGroup(agent.GroupOrderManage, agent.GroupMeta{
			Title:      "订单管理",
			IntentHint: "查看订单、取消订单、退款、订单状态",
			Keywords:   []string{"订单", "取消", "退款", "状态", "cancel", "refund", "order status", "查看订单", "订单号", "物流"},
		})

		registry.Register(agent.ToolEntry{Tool: searchBooksTool, Group: agent.GroupDiscovery, Name: "search_books", Description: "搜索书库中的书籍。当用户询问某类书时首先使用。"})
		registry.Register(agent.ToolEntry{Tool: getBookDetailTool, Group: agent.GroupDiscovery, Name: "get_book_detail", Description: "获取某本书的详细信息。"})
		registry.Register(agent.ToolEntry{Tool: checkStockTool, Group: agent.GroupDiscovery, Name: "check_stock", Description: "确认特定门店的库存。"})
		registry.Register(agent.ToolEntry{Tool: findSimilarTool, Group: agent.GroupDiscovery, Name: "find_similar_books", Description: "当用户问到「类似」某本书的书时使用。"})
		registry.Register(agent.ToolEntry{Tool: getUserOrdersTool, Group: agent.GroupOrderManage, Name: "get_user_orders", Description: "查看用户的购买历史，了解其阅读偏好。也可以用于查看最近的订单。"})
		registry.Register(agent.ToolEntry{Tool: addToCartTool, Group: agent.GroupShopping, Name: "add_to_cart", Description: "当用户明确表示想购买某本书时，帮助他们加入购物车。在加购前先确认用户的意图。"})
		registry.Register(agent.ToolEntry{Tool: getCartTool, Group: agent.GroupShopping, Name: "get_cart", Description: "查看用户的购物车内容、总数和总金额。"})
		registry.Register(agent.ToolEntry{Tool: createOrderTool, Group: agent.GroupOrder, Name: "create_order", Description: "为用户创建订单。必须先展示订单摘要并获得用户确认后才能调用。", Sensitive: true})
		registry.Register(agent.ToolEntry{Tool: getOrderDetailTool, Group: agent.GroupOrderManage, Name: "get_order_detail", Description: "查询某个订单的状态和详情。"})
		registry.Register(agent.ToolEntry{Tool: cancelOrderTool, Group: agent.GroupOrder, Name: "cancel_order", Description: "取消订单。必须先告知用户取消后果并获得确认。", Sensitive: true})
		registry.Register(agent.ToolEntry{Tool: createPaymentTool, Group: agent.GroupOrder, Name: "create_payment", Description: "为订单创建支付。必须先告知用户支付金额和方式并获得确认。", Sensitive: true})
		log.Printf("ToolRegistry initialized with %d tools in 4 groups", registry.Count())

		intentRouter := agent.NewIntentRouter(registry, chatModelAnalysis)

		recommenderAgent, err := agent.NewRecommenderAgent(ctx, chatModelAnalysis)
		if err != nil {
			log.Fatalf("failed to create recommender agent: %v", err)
		}

		summarizerAgent, err := agent.NewSummarizerAgent(ctx, chatModelAnalysis, getBookDetailTool)
		if err != nil {
			log.Fatalf("failed to create summarizer agent: %v", err)
		}

		smartSearchAgent, err := agent.NewSmartSearchAgent(ctx, chatModelAnalysis, searchBooksTool)
		if err != nil {
			log.Fatalf("failed to create smart search agent: %v", err)
		}

		tasteAgent, err := agent.NewTasteAnalyzerAgent(ctx, chatModelAnalysis)
		if err != nil {
			log.Fatalf("failed to create taste analyzer agent: %v", err)
		}

		// RAG retriever: vector search + real-time inventory check
		bookRetriever := rag.NewBookRetriever(embSvc, store, inventoryClient)
		log.Println("RAG BookRetriever initialized")

		h = handler.NewAIHandler(repo, rdb, hasAPIKey, embSvc, bookRetriever, orderClient,
			chatModelConversation, registry, intentRouter,
			recommenderAgent, summarizerAgent, smartSearchAgent, tasteAgent)
		log.Println("all Eino agents initialized (dynamic tool loading, RAG-enabled)")

		// Background: generate embeddings for all books that don't have one yet
		go embSvc.EmbedAllBooks(context.Background())
	} else {
		log.Println("WARNING: OpenAI API key not set, AI features will return fallback responses")
		h = handler.NewAIHandler(repo, rdb, false, nil, nil, orderClient, nil, nil, agent.NewIntentRouter(nil, nil), nil, nil, nil, nil)
	}

	if err := pb.RegisterAIServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	log.Println("starting bookhive.ai service v3.0 (Eino framework, dynamic tool loading)...")
	if err := svc.Run(); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}
