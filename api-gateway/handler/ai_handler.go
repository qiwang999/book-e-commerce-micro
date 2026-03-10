package handler

import (
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
