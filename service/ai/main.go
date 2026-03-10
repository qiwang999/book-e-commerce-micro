package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
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

	// Milvus vector database
	milvusAddr := cfg.Milvus.Address
	if milvusAddr == "" {
		milvusAddr = "127.0.0.1:19530"
	}
	milvusStore, err := vectorstore.NewMilvusStore(ctx, milvusAddr)
	if err != nil {
		log.Fatalf("failed to connect to Milvus: %v", err)
	}
	defer milvusStore.Close()
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
		// Different temperatures for different purposes:
		// - Librarian (0.7): creative conversational, needs warmth
		// - Recommender (0.4): more deterministic, consistent recommendations
		// - Summarizer (0.2): factual, structured output
		// - SmartSearch (0.1): near-deterministic intent extraction
		// - TasteAnalyzer (0.5): analytical with some creative flair
		chatModelCreative, err := agent.NewChatModelWithTemperature(ctx, &cfg.OpenAI, 0.7)
		if err != nil {
			log.Fatalf("failed to create creative chatmodel: %v", err)
		}
		chatModelBalanced, err := agent.NewChatModelWithTemperature(ctx, &cfg.OpenAI, 0.4)
		if err != nil {
			log.Fatalf("failed to create balanced chatmodel: %v", err)
		}
		chatModelPrecise, err := agent.NewChatModelWithTemperature(ctx, &cfg.OpenAI, 0.2)
		if err != nil {
			log.Fatalf("failed to create precise chatmodel: %v", err)
		}
		chatModelDeterministic, err := agent.NewChatModelWithTemperature(ctx, &cfg.OpenAI, 0.1)
		if err != nil {
			log.Fatalf("failed to create deterministic chatmodel: %v", err)
		}
		log.Println("Eino ChatModels (OpenAI) initialized with varied temperatures")

		// Embedding service (Eino embedder + Milvus vector store)
		embSvc, err := embedding.NewService(ctx, &cfg.OpenAI, milvusStore, bookClient)
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

		librarianAgent, err := agent.NewLibrarianAgent(ctx, chatModelCreative, []tool.BaseTool{
			searchBooksTool, getBookDetailTool, checkStockTool, findSimilarTool,
			getUserOrdersTool, addToCartTool,
		})
		if err != nil {
			log.Fatalf("failed to create librarian agent: %v", err)
		}

		recommenderAgent, err := agent.NewRecommenderAgent(ctx, chatModelBalanced)
		if err != nil {
			log.Fatalf("failed to create recommender agent: %v", err)
		}

		summarizerAgent, err := agent.NewSummarizerAgent(ctx, chatModelPrecise, getBookDetailTool)
		if err != nil {
			log.Fatalf("failed to create summarizer agent: %v", err)
		}

		smartSearchAgent, err := agent.NewSmartSearchAgent(ctx, chatModelDeterministic, searchBooksTool)
		if err != nil {
			log.Fatalf("failed to create smart search agent: %v", err)
		}

		tasteAgent, err := agent.NewTasteAnalyzerAgent(ctx, chatModelBalanced)
		if err != nil {
			log.Fatalf("failed to create taste analyzer agent: %v", err)
		}

		// RAG retriever: Milvus vector search + real-time inventory check
		bookRetriever := rag.NewBookRetriever(embSvc, milvusStore, inventoryClient)
		log.Println("RAG BookRetriever initialized")

		h = handler.NewAIHandler(repo, rdb, hasAPIKey, embSvc, bookRetriever, orderClient, librarianAgent, recommenderAgent, summarizerAgent, smartSearchAgent, tasteAgent)
		log.Println("all Eino agents initialized (RAG-enabled)")

		// Background: generate embeddings for all books that don't have one yet
		go embSvc.EmbedAllBooks(context.Background())
	} else {
		log.Println("WARNING: OpenAI API key not set, AI features will return fallback responses")
		h = handler.NewAIHandler(repo, rdb, false, nil, nil, orderClient, nil, nil, nil, nil, nil)
	}

	if err := pb.RegisterAIServiceHandler(svc.Server(), h); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	log.Println("starting bookhive.ai service v2.0 (Eino framework)...")
	if err := svc.Run(); err != nil {
		log.Fatalf("service exited with error: %v", err)
	}
}
