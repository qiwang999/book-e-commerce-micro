package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	aiPb "github.com/qiwang/book-e-commerce-micro/proto/ai"
)

func (h *Handlers) GetRecommendationsHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req aiPb.RecommendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	resp, err := h.AI.GetRecommendations(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) ChatWithLibrarianHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req aiPb.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	resp, err := h.AI.ChatWithLibrarian(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

// StreamChatHandler handles POST /ai/chat/stream — SSE streaming chat.
// The client sends a JSON body identical to ChatWithLibrarian, and receives
// Server-Sent Events with incremental tokens, metadata, and a final "done".
//
// SSE event types:
//
//	event: delta      — data: {"delta":"..."}
//	event: metadata   — data: {"session_id":"...", "suggested_books":[...], "actions":[...]}
//	event: error      — data: {"error":"..."}
//	event: done       — data: [DONE]
func (h *Handlers) StreamChatHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req aiPb.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	stream, err := h.AI.StreamChat(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		default:
		}

		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[Gateway] StreamChat recv error: %v", err)
			writeSSE(c, "error", map[string]string{"error": "stream interrupted"})
			break
		}

		switch chunk.Type {
		case "delta":
			writeSSE(c, "delta", map[string]string{"delta": chunk.Delta})
		case "metadata":
			writeSSE(c, "metadata", map[string]interface{}{
				"session_id":      chunk.SessionId,
				"suggested_books": chunk.SuggestedBooks,
				"actions":         chunk.Actions,
			})
		case "error":
			writeSSE(c, "error", map[string]string{"error": chunk.Error})
		case "done":
			fmt.Fprintf(c.Writer, "event: done\ndata: [DONE]\n\n")
			c.Writer.Flush()
			return
		}
	}

	fmt.Fprintf(c.Writer, "event: done\ndata: [DONE]\n\n")
	c.Writer.Flush()
}

func writeSSE(c *gin.Context, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, jsonData)
	c.Writer.Flush()
}

func (h *Handlers) GenerateBookSummaryHandler(c *gin.Context) {
	bookID := c.Param("book_id")

	resp, err := h.AI.GenerateBookSummary(c.Request.Context(), &aiPb.SummaryRequest{
		BookId: bookID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) SmartSearchHandler(c *gin.Context) {
	var req aiPb.SmartSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.AI.SmartSearch(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) AnalyzeReadingTasteHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")

	resp, err := h.AI.AnalyzeReadingTaste(c.Request.Context(), &aiPb.TasteRequest{
		UserId: userID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetSimilarBooksHandler(c *gin.Context) {
	bookID := c.Param("book_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit < 1 || limit > 50 {
		limit = 5
	}

	resp, err := h.AI.GetSimilarBooks(c.Request.Context(), &aiPb.SimilarBooksRequest{
		BookId: bookID,
		Limit:  int32(limit),
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}
