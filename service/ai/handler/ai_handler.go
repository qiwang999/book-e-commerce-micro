package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	pb "github.com/qiwang/book-e-commerce-micro/proto/ai"
	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
	"github.com/qiwang/book-e-commerce-micro/service/ai/agent"
	"github.com/qiwang/book-e-commerce-micro/service/ai/embedding"
	"github.com/qiwang/book-e-commerce-micro/service/ai/hitl"
	"github.com/qiwang/book-e-commerce-micro/service/ai/model"
	"github.com/qiwang/book-e-commerce-micro/service/ai/rag"
	"github.com/qiwang/book-e-commerce-micro/service/ai/repository"
)

const (
	agentTimeout       = 60 * time.Second
	agentChatTimeout   = 90 * time.Second
	redisCacheTTL      = 24 * time.Hour
	redisCacheShortTTL = 1 * time.Hour
)

type AIHandler struct {
	repo         *repository.AIRepository
	rdb          *redis.Client
	hasAPIKey    bool
	embSvc       *embedding.Service
	retriever    *rag.BookRetriever
	orderSvc     orderPb.OrderService
	chatModel    einoModel.ToolCallingChatModel
	toolRegistry *agent.ToolRegistry
	intentRouter *agent.IntentRouter
	recommender  adk.Agent
	summarizer   adk.Agent
	searcher     adk.Agent
	taster       adk.Agent
}

func NewAIHandler(
	repo *repository.AIRepository,
	rdb *redis.Client,
	hasAPIKey bool,
	embSvc *embedding.Service,
	retriever *rag.BookRetriever,
	orderSvc orderPb.OrderService,
	chatModel einoModel.ToolCallingChatModel,
	toolRegistry *agent.ToolRegistry,
	intentRouter *agent.IntentRouter,
	recommender, summarizer, searcher, taster adk.Agent,
) *AIHandler {
	return &AIHandler{
		repo:         repo,
		rdb:          rdb,
		hasAPIKey:    hasAPIKey,
		embSvc:       embSvc,
		retriever:    retriever,
		orderSvc:     orderSvc,
		chatModel:    chatModel,
		toolRegistry: toolRegistry,
		intentRouter: intentRouter,
		recommender:  recommender,
		summarizer:   summarizer,
		searcher:     searcher,
		taster:       taster,
	}
}

func (h *AIHandler) GetRecommendations(ctx context.Context, req *pb.RecommendRequest, rsp *pb.RecommendResponse) error {
	if !h.hasAPIKey {
		rsp.Recommendations = fallbackRecommendations()
		return nil
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	// RAG: retrieve relevant books from Milvus with stock info
	ragContext := ""
	if h.retriever != nil && req.Context != "" {
		ragCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		docs, err := h.retriever.Retrieve(ragCtx, req.Context, int(limit*2))
		cancel()
		if err != nil {
			log.Printf("[AI] GetRecommendations RAG retrieve error: %v", err)
		} else {
			ragContext = rag.FormatDocsAsContext(docs)
		}
	}

	history := h.fetchUserPurchaseHistory(ctx, req.UserId)

	prompt := fmt.Sprintf("User ID: %d\nContext: %s\nPlease recommend %d books.", req.UserId, req.Context, limit)
	if history != "" {
		prompt = history + "\n" + prompt + "\nIMPORTANT: Recommend books that complement the user's existing collection. Avoid recommending books they've already purchased. Ensure diversity across categories."
	}
	if ragContext != "" {
		prompt = ragContext + "\n\n" + prompt
	}
	reply, err := h.runAgent(ctx, h.recommender, prompt)
	if err != nil {
		log.Printf("[AI] GetRecommendations agent error: %v", err)
		rsp.Recommendations = fallbackRecommendations()
		return nil
	}

	var recs []struct {
		BookID   string  `json:"book_id"`
		Title    string  `json:"title"`
		Author   string  `json:"author"`
		Category string  `json:"category"`
		Score    float64 `json:"score"`
		Reason   string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(cleanJSONResponse(reply)), &recs); err != nil {
		log.Printf("[AI] GetRecommendations json parse error: %v, raw: %s", err, truncate(reply, 200))
		rsp.Recommendations = fallbackRecommendations()
		return nil
	}

	for _, r := range recs {
		rsp.Recommendations = append(rsp.Recommendations, &pb.BookRecommendation{
			BookId:   r.BookID,
			Title:    r.Title,
			Author:   r.Author,
			Category: r.Category,
			Score:    r.Score,
			Reason:   r.Reason,
		})
	}
	return nil
}

func (h *AIHandler) ChatWithLibrarian(ctx context.Context, req *pb.ChatRequest, rsp *pb.ChatResponse) error {
	sessionID := req.SessionId
	if sessionID == "" {
		sessionID = uuid.New().String()
	}
	rsp.SessionId = sessionID

	if req.HitlConfirmActionId != "" && h.rdb != nil {
		if err := hitl.ApplyConfirmation(ctx, h.rdb, req.UserId, sessionID, req.HitlConfirmActionId, req.HitlConfirmSecret); err != nil {
			log.Printf("[AI] HITL confirm error: %v", err)
		}
	}

	if !h.hasAPIKey {
		rsp.Reply = "I'm the BookHive AI Librarian. Our AI service is currently being configured. Please try again later!"
		return nil
	}

	session, err := h.repo.LoadChatSession(ctx, sessionID)
	if err != nil {
		log.Printf("[AI] ChatWithLibrarian load session error: %v", err)
	}
	if session == nil {
		session = &model.ChatSession{
			SessionID: sessionID,
			UserID:    req.UserId,
			CreatedAt: time.Now(),
		}
	}

	session.Messages = append(session.Messages, model.ChatMessage{
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	})

	// RAG: only retrieve when the message looks like a book-related query
	ragContext := ""
	if h.retriever != nil && shouldRetrieve(req.Message) {
		docs, err := h.retriever.Retrieve(ctx, req.Message, 8)
		if err != nil {
			log.Printf("[AI] ChatWithLibrarian RAG retrieve error: %v", err)
		} else {
			ragContext = rag.FormatDocsAsContext(docs)
		}
	}

	inputMsgs := h.buildInputMessages(session)

	// Inject RAG context into the latest user message so it's always adjacent
	// to the user's query, avoiding context dilution from long conversation history.
	if ragContext != "" && len(inputMsgs) > 0 {
		last := inputMsgs[len(inputMsgs)-1]
		if last.Role == schema.User {
			last.Content = ragContext + "\n\n" + last.Content
		} else {
			inputMsgs = append(inputMsgs, &schema.Message{
				Role:    schema.System,
				Content: ragContext,
			})
		}
	}

	recentHistory := h.extractRecentHistory(session)
	groups := h.intentRouter.Classify(ctx, req.Message, recentHistory)

	if h.recommender != nil && hitl.ShouldRunRecommenderSubAgent(req.Message, toolGroupStrings(groups)) {
		hist := h.fetchUserPurchaseHistory(ctx, req.UserId)
		prompt := fmt.Sprintf("User message:\n%s\n\n%s\nReply with JSON array only (objects: book_id, title, author, category, score, reason), at most 5 items, matching the user's interests.", req.Message, hist)
		subOut, err := h.runAgent(ctx, h.recommender, prompt)
		if err != nil {
			log.Printf("[AI] recommender sub-agent error: %v", err)
		} else if strings.TrimSpace(subOut) != "" {
			prefix := "【内部：推荐子代理初稿；请结合用户问题筛选、用书库工具核实后再回复；勿向用户逐字复述本段】\n" + subOut
			inputMsgs = append([]*schema.Message{{Role: schema.System, Content: prefix}}, inputMsgs...)
		}
	}

	librarian, err := agent.NewLibrarianAgent(ctx, h.chatModel, h.toolRegistry, groups)
	if err != nil {
		log.Printf("[AI] ChatWithLibrarian create agent error: %v", err)
		rsp.Reply = "Sorry, I'm having trouble thinking right now. Please try again in a moment."
		return nil
	}

	gate := hitl.NewGate(h.rdb)
	chatCtx := hitl.WithGate(hitl.WithChatMeta(ctx, req.UserId, sessionID), gate)
	reply, err := h.runAgentWithMessages(chatCtx, librarian, inputMsgs)
	if err != nil {
		log.Printf("[AI] ChatWithLibrarian agent error: %v", err)
		rsp.Reply = "Sorry, I'm having trouble thinking right now. Please try again in a moment."
		return nil
	}

	rsp.Reply = reply
	rsp.SuggestedBooks = parseSuggestedBooks(reply)
	rsp.Actions = parseActions(reply)
	if p := gate.TakeLastPending(); p != nil {
		rsp.HitlPending = true
		rsp.HitlActionId = p.ActionID
		rsp.HitlSecret = p.Secret
		rsp.HitlSummary = p.Summary
	}

	session.Messages = append(session.Messages, model.ChatMessage{
		Role:      "assistant",
		Content:   reply,
		CreatedAt: time.Now(),
	})
	session.UpdatedAt = time.Now()

	if err := h.repo.SaveChatSession(ctx, session); err != nil {
		log.Printf("[AI] ChatWithLibrarian save session error: %v", err)
	}

	return nil
}

func (h *AIHandler) GenerateBookSummary(ctx context.Context, req *pb.SummaryRequest, rsp *pb.SummaryResponse) error {
	cacheKey := fmt.Sprintf("ai:summary:%s", req.BookId)

	if !req.ForceRegenerate {
		if raw, ok := h.cacheGet(ctx, cacheKey); ok {
			var cached struct {
				BookID            string   `json:"book_id"`
				Title             string   `json:"title"`
				Summary           string   `json:"summary"`
				KeyThemes         []string `json:"key_themes"`
				TargetAudience    string   `json:"target_audience"`
				ReadingDifficulty string   `json:"reading_difficulty"`
				EstReadingHours   int32    `json:"estimated_reading_hours"`
			}
			if json.Unmarshal([]byte(raw), &cached) == nil {
				rsp.BookId = cached.BookID
				rsp.Title = cached.Title
				rsp.Summary = cached.Summary
				rsp.KeyThemes = cached.KeyThemes
				rsp.TargetAudience = cached.TargetAudience
				rsp.ReadingDifficulty = cached.ReadingDifficulty
				rsp.EstimatedReadingHours = cached.EstReadingHours
				return nil
			}
		}

		cached, err := h.repo.GetBookSummary(ctx, req.BookId)
		if err != nil {
			log.Printf("[AI] GenerateBookSummary cache lookup error: %v", err)
		}
		if cached != nil {
			rsp.BookId = cached.BookID
			rsp.Title = cached.Title
			rsp.Summary = cached.Summary
			rsp.KeyThemes = cached.KeyThemes
			rsp.TargetAudience = cached.TargetAudience
			rsp.ReadingDifficulty = cached.ReadingDifficulty
			rsp.EstimatedReadingHours = cached.EstReadingHours
			if data, err := json.Marshal(rsp); err == nil {
				h.cacheSet(ctx, cacheKey, string(data), redisCacheTTL)
			}
			return nil
		}
	}

	if !h.hasAPIKey {
		rsp.BookId = req.BookId
		rsp.Summary = "AI summary generation is currently unavailable. Please try again later."
		return nil
	}

	prompt := fmt.Sprintf("Generate a structured summary for the book with ID: %s", req.BookId)
	reply, err := h.runAgent(ctx, h.summarizer, prompt)
	if err != nil {
		log.Printf("[AI] GenerateBookSummary agent error: %v", err)
		rsp.BookId = req.BookId
		rsp.Summary = "Failed to generate summary. Please try again."
		return nil
	}

	var parsed struct {
		Title             string   `json:"title"`
		Summary           string   `json:"summary"`
		KeyThemes         []string `json:"key_themes"`
		TargetAudience    string   `json:"target_audience"`
		ReadingDifficulty string   `json:"reading_difficulty"`
		EstReadingHours   int32    `json:"estimated_reading_hours"`
	}
	if err := json.Unmarshal([]byte(cleanJSONResponse(reply)), &parsed); err != nil {
		log.Printf("[AI] GenerateBookSummary json parse error: %v, raw: %s", err, truncate(reply, 200))
		rsp.BookId = req.BookId
		rsp.Summary = reply
		return nil
	}

	rsp.BookId = req.BookId
	rsp.Title = parsed.Title
	rsp.Summary = parsed.Summary
	rsp.KeyThemes = parsed.KeyThemes
	rsp.TargetAudience = parsed.TargetAudience
	rsp.ReadingDifficulty = parsed.ReadingDifficulty
	rsp.EstimatedReadingHours = parsed.EstReadingHours

	cache := &model.BookSummaryCache{
		BookID:            req.BookId,
		Title:             parsed.Title,
		Summary:           parsed.Summary,
		KeyThemes:         parsed.KeyThemes,
		TargetAudience:    parsed.TargetAudience,
		ReadingDifficulty: parsed.ReadingDifficulty,
		EstReadingHours:   parsed.EstReadingHours,
		CreatedAt:         time.Now(),
	}
	if err := h.repo.SaveBookSummary(ctx, cache); err != nil {
		log.Printf("[AI] GenerateBookSummary cache save error: %v", err)
	}

	if data, err := json.Marshal(rsp); err == nil {
		h.cacheSet(ctx, cacheKey, string(data), redisCacheTTL)
	}

	return nil
}

func (h *AIHandler) SmartSearch(ctx context.Context, req *pb.SmartSearchRequest, rsp *pb.SmartSearchResponse) error {
	if !h.hasAPIKey {
		rsp.InterpretedQuery = req.Query
		rsp.ExtractedFilters = map[string]string{"keyword": req.Query}
		return nil
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	reply, err := h.runAgent(ctx, h.searcher, req.Query)
	if err != nil {
		log.Printf("[AI] SmartSearch agent error: %v", err)
		rsp.InterpretedQuery = req.Query
		rsp.ExtractedFilters = map[string]string{"keyword": req.Query}
		return nil
	}

	var parsed struct {
		InterpretedQuery string            `json:"interpreted_query"`
		Filters          map[string]string `json:"filters"`
		Results          []struct {
			BookID   string  `json:"book_id"`
			Title    string  `json:"title"`
			Author   string  `json:"author"`
			Category string  `json:"category"`
			Price    float64 `json:"price"`
			Score    float64 `json:"score"`
			Reason   string  `json:"reason"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(cleanJSONResponse(reply)), &parsed); err != nil {
		log.Printf("[AI] SmartSearch json parse error: %v, raw: %s", err, truncate(reply, 200))
		rsp.InterpretedQuery = req.Query
		rsp.ExtractedFilters = map[string]string{"keyword": req.Query}
		return nil
	}

	rsp.InterpretedQuery = parsed.InterpretedQuery
	rsp.ExtractedFilters = parsed.Filters

	for i, r := range parsed.Results {
		if i >= limit {
			break
		}
		rsp.Results = append(rsp.Results, &pb.BookRecommendation{
			BookId:   r.BookID,
			Title:    r.Title,
			Author:   r.Author,
			Category: r.Category,
			Price:    r.Price,
			Score:    r.Score,
			Reason:   r.Reason,
		})
	}
	return nil
}

// fetchUserPurchaseHistory fetches the user's recent completed orders and formats
// them as context for LLM consumption.
func (h *AIHandler) fetchUserPurchaseHistory(ctx context.Context, userID uint64) string {
	if h.orderSvc == nil {
		return ""
	}

	listCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	resp, err := h.orderSvc.ListOrders(listCtx, &orderPb.ListOrdersRequest{
		UserId:   userID,
		Status:   "completed",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		log.Printf("[AI] fetchUserPurchaseHistory error: %v", err)
		return ""
	}
	if resp == nil || len(resp.Orders) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("=== 用户购买历史 ===\n")
	seen := make(map[string]int)
	for _, order := range resp.Orders {
		for _, item := range order.Items {
			key := item.BookId
			if key == "" {
				key = item.BookTitle
			}
			seen[key]++
			b.WriteString(fmt.Sprintf("- 《%s》 by %s (数量: %d)\n", item.BookTitle, item.BookAuthor, item.Quantity))
		}
	}
	b.WriteString(fmt.Sprintf("共 %d 个订单，涉及 %d 种不同书籍\n", len(resp.Orders), len(seen)))
	b.WriteString("=== 购买历史结束 ===\n")
	return b.String()
}

func (h *AIHandler) AnalyzeReadingTaste(ctx context.Context, req *pb.TasteRequest, rsp *pb.TasteResponse) error {
	rsp.UserId = req.UserId

	if !h.hasAPIKey {
		rsp.TasteSummary = "AI taste analysis is currently unavailable. Please try again later."
		return nil
	}

	history := h.fetchUserPurchaseHistory(ctx, req.UserId)
	prompt := fmt.Sprintf("Analyze the reading taste for user ID: %d.\n\n%s\nBased on the purchase history above (if available), provide a detailed taste analysis. If no purchase history is available, provide a general analysis and suggest the user to make some purchases first.", req.UserId, history)
	reply, err := h.runAgent(ctx, h.taster, prompt)
	if err != nil {
		log.Printf("[AI] AnalyzeReadingTaste agent error: %v", err)
		rsp.TasteSummary = "Unable to analyze reading taste at this time."
		return nil
	}

	var parsed struct {
		TopCategories       []string `json:"top_categories"`
		TopAuthors          []string `json:"top_authors"`
		PersonalityTags     []string `json:"personality_tags"`
		TasteSummary        string   `json:"taste_summary"`
		DiscoverySuggestions []struct {
			Title    string `json:"title"`
			Author   string `json:"author"`
			Category string `json:"category"`
			Reason   string `json:"reason"`
		} `json:"discovery_suggestions"`
	}
	if err := json.Unmarshal([]byte(cleanJSONResponse(reply)), &parsed); err != nil {
		log.Printf("[AI] AnalyzeReadingTaste json parse error: %v, raw: %s", err, truncate(reply, 200))
		rsp.TasteSummary = reply
		return nil
	}

	rsp.TopCategories = parsed.TopCategories
	rsp.TopAuthors = parsed.TopAuthors
	rsp.PersonalityTags = parsed.PersonalityTags
	rsp.TasteSummary = parsed.TasteSummary

	for _, s := range parsed.DiscoverySuggestions {
		rsp.DiscoverySuggestions = append(rsp.DiscoverySuggestions, &pb.BookRecommendation{
			Title:    s.Title,
			Author:   s.Author,
			Category: s.Category,
			Reason:   s.Reason,
		})
	}
	return nil
}

func (h *AIHandler) GetSimilarBooks(ctx context.Context, req *pb.SimilarBooksRequest, rsp *pb.SimilarBooksResponse) error {
	rsp.BookId = req.BookId

	if h.embSvc == nil {
		log.Printf("[AI] GetSimilarBooks: embedding service not available")
		return nil
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 5
	}

	similar, err := h.embSvc.FindSimilarBooks(ctx, req.BookId, limit)
	if err != nil {
		log.Printf("[AI] GetSimilarBooks error: %v", err)
		return nil
	}

	for _, s := range similar {
		rsp.SimilarBooks = append(rsp.SimilarBooks, &pb.BookRecommendation{
			BookId:   s.BookID,
			Title:    s.Title,
			Author:   s.Author,
			Category: s.Category,
			Score:    s.Score,
			Reason:   fmt.Sprintf("Semantic similarity score: %.2f", s.Score),
		})
	}
	return nil
}

func (h *AIHandler) BackfillEmbeddings(ctx context.Context, req *pb.BackfillEmbeddingsRequest, rsp *pb.BackfillEmbeddingsResponse) error {
	if h.embSvc == nil || !h.hasAPIKey {
		return fmt.Errorf("embedding backfill requires OpenAI API key and vector store")
	}

	opts := embedding.EmbedAllBooksOptions{
		Force:         req.Force,
		MaxEmbeddings: int(req.SyncLimit),
		MaxErrors:     int(req.MaxErrors),
	}

	if req.SyncLimit == 0 {
		go func() {
			stats := h.embSvc.EmbedAllBooks(context.Background(), opts)
			log.Printf("[embedding] BackfillEmbeddings background done: embedded=%d skipped=%d errors=%d",
				stats.Embedded, stats.Skipped, stats.Errors)
		}()
		rsp.Background = true
		rsp.Message = "全量向量回填已在后台执行，请查看 AI 服务日志 [embedding]"
		return nil
	}

	stats := h.embSvc.EmbedAllBooks(ctx, opts)
	rsp.Embedded = int32(stats.Embedded)
	rsp.Skipped = int32(stats.Skipped)
	rsp.Errors = int32(stats.Errors)
	rsp.Message = fmt.Sprintf("同步回填完成: embedded=%d skipped=%d errors=%d", stats.Embedded, stats.Skipped, stats.Errors)
	return nil
}

func (h *AIHandler) StreamChat(ctx context.Context, req *pb.ChatRequest, stream pb.AIService_StreamChatStream) error {
	sessionID := req.SessionId
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	if err := stream.Send(&pb.ChatStreamChunk{Type: "metadata", SessionId: sessionID}); err != nil {
		return err
	}

	if req.HitlConfirmActionId != "" && h.rdb != nil {
		if err := hitl.ApplyConfirmation(ctx, h.rdb, req.UserId, sessionID, req.HitlConfirmActionId, req.HitlConfirmSecret); err != nil {
			log.Printf("[AI] StreamChat HITL confirm error: %v", err)
		}
	}

	if !h.hasAPIKey {
		_ = stream.Send(&pb.ChatStreamChunk{Type: "delta", Delta: "I'm the BookHive AI Librarian. Our AI service is currently being configured. Please try again later!"})
		_ = stream.Send(&pb.ChatStreamChunk{Type: "done"})
		return nil
	}

	session, err := h.repo.LoadChatSession(ctx, sessionID)
	if err != nil {
		log.Printf("[AI] StreamChat load session error: %v", err)
	}
	if session == nil {
		session = &model.ChatSession{
			SessionID: sessionID,
			UserID:    req.UserId,
			CreatedAt: time.Now(),
		}
	}

	session.Messages = append(session.Messages, model.ChatMessage{
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	})

	ragContext := ""
	if h.retriever != nil && shouldRetrieve(req.Message) {
		docs, err := h.retriever.Retrieve(ctx, req.Message, 8)
		if err != nil {
			log.Printf("[AI] StreamChat RAG retrieve error: %v", err)
		} else {
			ragContext = rag.FormatDocsAsContext(docs)
		}
	}

	inputMsgs := h.buildInputMessages(session)

	if ragContext != "" && len(inputMsgs) > 0 {
		last := inputMsgs[len(inputMsgs)-1]
		if last.Role == schema.User {
			last.Content = ragContext + "\n\n" + last.Content
		} else {
			inputMsgs = append(inputMsgs, &schema.Message{
				Role:    schema.System,
				Content: ragContext,
			})
		}
	}

	recentHistory := h.extractRecentHistory(session)
	groups := h.intentRouter.Classify(ctx, req.Message, recentHistory)

	if h.recommender != nil && hitl.ShouldRunRecommenderSubAgent(req.Message, toolGroupStrings(groups)) {
		hist := h.fetchUserPurchaseHistory(ctx, req.UserId)
		prompt := fmt.Sprintf("User message:\n%s\n\n%s\nReply with JSON array only (objects: book_id, title, author, category, score, reason), at most 5 items, matching the user's interests.", req.Message, hist)
		subOut, err := h.runAgent(ctx, h.recommender, prompt)
		if err != nil {
			log.Printf("[AI] StreamChat recommender sub-agent error: %v", err)
		} else if strings.TrimSpace(subOut) != "" {
			prefix := "【内部：推荐子代理初稿；请结合用户问题筛选、用书库工具核实后再回复；勿向用户逐字复述本段】\n" + subOut
			inputMsgs = append([]*schema.Message{{Role: schema.System, Content: prefix}}, inputMsgs...)
		}
	}

	librarian, err := agent.NewLibrarianAgent(ctx, h.chatModel, h.toolRegistry, groups)
	if err != nil {
		log.Printf("[AI] StreamChat create agent error: %v", err)
		_ = stream.Send(&pb.ChatStreamChunk{Type: "error", Error: "Failed to initialize agent"})
		_ = stream.Send(&pb.ChatStreamChunk{Type: "done"})
		return nil
	}

	gate := hitl.NewGate(h.rdb)
	chatBase := hitl.WithGate(hitl.WithChatMeta(ctx, req.UserId, sessionID), gate)

	chatCtx, cancel := context.WithTimeout(chatBase, agentChatTimeout)
	defer cancel()

	runner := adk.NewRunner(chatCtx, adk.RunnerConfig{
		Agent:          librarian,
		EnableStreaming: true,
	})
	iter := runner.Run(chatCtx, inputMsgs)

	var fullReply strings.Builder
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			log.Printf("[AI] StreamChat agent error: %v", event.Err)
			_ = stream.Send(&pb.ChatStreamChunk{Type: "error", Error: "Agent encountered an error"})
			break
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput
		if mv.Role != schema.Assistant {
			continue
		}

		if mv.IsStreaming && mv.MessageStream != nil {
			for {
				msg, err := mv.MessageStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("[AI] StreamChat stream recv error: %v", err)
					break
				}
				if msg != nil && msg.Content != "" {
					fullReply.WriteString(msg.Content)
					if sendErr := stream.Send(&pb.ChatStreamChunk{Type: "delta", Delta: msg.Content}); sendErr != nil {
						log.Printf("[AI] StreamChat send error: %v", sendErr)
						return sendErr
					}
				}
			}
		} else if mv.Message != nil && mv.Message.Content != "" {
			fullReply.WriteString(mv.Message.Content)
			if sendErr := stream.Send(&pb.ChatStreamChunk{Type: "delta", Delta: mv.Message.Content}); sendErr != nil {
				return sendErr
			}
		}
	}

	reply := fullReply.String()
	if p := gate.TakeLastPending(); p != nil {
		_ = stream.Send(&pb.ChatStreamChunk{
			Type:          "metadata",
			SessionId:   sessionID,
			HitlPending: true,
			HitlActionId: p.ActionID,
			HitlSecret:   p.Secret,
			HitlSummary:  p.Summary,
		})
	}
	if reply != "" {
		suggestedBooks := parseSuggestedBooks(reply)
		actions := parseActions(reply)
		if len(suggestedBooks) > 0 || len(actions) > 0 {
			_ = stream.Send(&pb.ChatStreamChunk{
				Type:           "metadata",
				SessionId:      sessionID,
				SuggestedBooks: suggestedBooks,
				Actions:        actions,
			})
		}

		session.Messages = append(session.Messages, model.ChatMessage{
			Role:      "assistant",
			Content:   reply,
			CreatedAt: time.Now(),
		})
		session.UpdatedAt = time.Now()

		if err := h.repo.SaveChatSession(ctx, session); err != nil {
			log.Printf("[AI] StreamChat save session error: %v", err)
		}
	}

	_ = stream.Send(&pb.ChatStreamChunk{Type: "done"})
	return nil
}

// extractRecentHistory returns the last few user messages from the session for
// intent classification context carry-over.
func toolGroupStrings(groups []agent.ToolGroup) []string {
	s := make([]string, len(groups))
	for i, g := range groups {
		s[i] = string(g)
	}
	return s
}

func (h *AIHandler) extractRecentHistory(session *model.ChatSession) []string {
	var history []string
	for i := len(session.Messages) - 1; i >= 0 && len(history) < 4; i-- {
		if session.Messages[i].Role == "user" {
			history = append([]string{session.Messages[i].Content}, history...)
		}
	}
	return history
}

// buildInputMessages extracts the last 20 messages from a session as Eino schema messages.
func (h *AIHandler) buildInputMessages(session *model.ChatSession) []*schema.Message {
	historyStart := 0
	if len(session.Messages) > 20 {
		historyStart = len(session.Messages) - 20
	}
	var msgs []*schema.Message
	for _, m := range session.Messages[historyStart:] {
		role := schema.User
		switch m.Role {
		case "assistant":
			role = schema.Assistant
		case "system":
			role = schema.System
		case "tool":
			role = schema.Tool
		}
		msgs = append(msgs, &schema.Message{
			Role:    role,
			Content: m.Content,
		})
	}
	return msgs
}

// runAgent sends a single text prompt to an agent and collects the final text reply.
func (h *AIHandler) runAgent(ctx context.Context, a adk.Agent, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, agentTimeout)
	defer cancel()

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: a})
	iter := runner.Query(ctx, prompt)
	return collectAgentReply(iter)
}

// runAgentWithMessages runs the agent with a pre-built message list.
func (h *AIHandler) runAgentWithMessages(ctx context.Context, a adk.Agent, msgs []*schema.Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, agentChatTimeout)
	defer cancel()

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: a})
	iter := runner.Run(ctx, msgs)
	return collectAgentReply(iter)
}

// cacheGet tries to retrieve a cached value from Redis.
func (h *AIHandler) cacheGet(ctx context.Context, key string) (string, bool) {
	if h.rdb == nil {
		return "", false
	}
	val, err := h.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// cacheSet stores a value in Redis with the given TTL.
func (h *AIHandler) cacheSet(ctx context.Context, key, value string, ttl time.Duration) {
	if h.rdb == nil {
		return
	}
	if err := h.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		log.Printf("[AI] redis cache set error for key %s: %v", key, err)
	}
}

func collectAgentReply(iter *adk.AsyncIterator[*adk.AgentEvent]) (string, error) {
	var lastContent string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", event.Err
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				continue
			}
			if msg != nil && msg.Role == schema.Assistant && msg.Content != "" {
				lastContent = msg.Content
			}
		}
	}

	if lastContent == "" {
		return "", fmt.Errorf("agent returned no content")
	}
	return lastContent, nil
}

func fallbackRecommendations() []*pb.BookRecommendation {
	return []*pb.BookRecommendation{
		{Title: "The Great Gatsby", Author: "F. Scott Fitzgerald", Category: "Fiction", Score: 0.9, Reason: "A timeless classic of American literature"},
		{Title: "Sapiens", Author: "Yuval Noah Harari", Category: "Non-Fiction", Score: 0.85, Reason: "A fascinating journey through human history"},
		{Title: "Atomic Habits", Author: "James Clear", Category: "Self-Help", Score: 0.8, Reason: "Practical strategies for building better habits"},
	}
}

func parseSuggestedBooks(reply string) []*pb.BookRecommendation {
	const tag = "[SUGGESTED_BOOKS]"
	idx := indexOf(reply, tag)
	if idx < 0 {
		return nil
	}
	jsonStr := extractJSONArray(reply[idx+len(tag):])
	if jsonStr == "" {
		return nil
	}

	var books []struct {
		BookID   string  `json:"book_id"`
		Title    string  `json:"title"`
		Author   string  `json:"author"`
		Category string  `json:"category"`
		Price    float64 `json:"price"`
		CoverURL string  `json:"cover_url"`
		Reason   string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &books); err != nil {
		return nil
	}

	var result []*pb.BookRecommendation
	for _, b := range books {
		result = append(result, &pb.BookRecommendation{
			BookId:   b.BookID,
			Title:    b.Title,
			Author:   b.Author,
			Category: b.Category,
			Price:    b.Price,
			CoverUrl: b.CoverURL,
			Reason:   b.Reason,
		})
	}
	return result
}

func parseActions(reply string) []*pb.ActionSuggestion {
	const tag = "[ACTIONS]"
	idx := indexOf(reply, tag)
	if idx < 0 {
		return nil
	}
	jsonStr := extractJSONArray(reply[idx+len(tag):])
	if jsonStr == "" {
		return nil
	}

	var actions []struct {
		Type    string          `json:"type"`
		Label   string          `json:"label"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &actions); err != nil {
		return nil
	}

	var result []*pb.ActionSuggestion
	for _, a := range actions {
		payload := string(a.Payload)
		// LLM may return payload as a JSON object or a quoted string; normalise to string.
		if len(payload) > 0 && payload[0] == '"' {
			var s string
			if json.Unmarshal(a.Payload, &s) == nil {
				payload = s
			}
		}
		result = append(result, &pb.ActionSuggestion{
			Type:    a.Type,
			Label:   a.Label,
			Payload: payload,
		})
	}
	return result
}

// shouldRetrieve returns false for short conversational messages that don't
// warrant a vector database lookup (greetings, acknowledgements, etc.).
func shouldRetrieve(msg string) bool {
	msg = strings.TrimSpace(msg)
	if len([]rune(msg)) < 4 {
		return false
	}
	lower := strings.ToLower(msg)
	skipPhrases := []string{
		"你好", "hello", "hi", "hey", "谢谢", "thanks", "thank you",
		"好的", "ok", "okay", "嗯", "对", "是的", "yes", "no", "不",
		"再见", "bye", "拜拜", "哈哈", "呵呵", "good", "great", "nice",
	}
	for _, p := range skipPhrases {
		if lower == p {
			return false
		}
	}
	return true
}

// cleanJSONResponse strips markdown code fences (```json ... ```) that LLMs
// often wrap around JSON output, leaving only the raw JSON content.
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)
	// Strip markdown code fences
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	// Already valid JSON
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		return s
	}
	// LLM may prepend free-text before the JSON — extract the outermost { } or [ ]
	return extractOutermostJSON(s)
}

// extractOutermostJSON finds the first top-level JSON object or array in s by
// scanning for an opening brace/bracket and matching its close via depth counting.
func extractOutermostJSON(s string) string {
	start := -1
	var open, close byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' || s[i] == '[' {
			start = i
			open = s[i]
			if open == '{' {
				close = '}'
			} else {
				close = ']'
			}
			break
		}
	}
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' && inStr {
			esc = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func extractJSONArray(s string) string {
	start := -1
	depth := 0
	for i, c := range s {
		switch c {
		case '[':
			if start == -1 {
				start = i
			}
			depth++
		case ']':
			depth--
			if depth == 0 && start >= 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
