package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	bookPb "github.com/qiwang/book-e-commerce-micro/proto/book"
)

func (h *Handlers) SearchBooksHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	req := &bookPb.SearchBooksRequest{
		Keyword:  c.Query("keyword"),
		Category: c.Query("category"),
		Author:   c.Query("author"),
		Language: c.Query("language"),
		SortBy:   c.Query("sort_by"),
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	if minPrice := c.Query("min_price"); minPrice != "" {
		req.MinPrice, _ = strconv.ParseFloat(minPrice, 64)
	}
	if maxPrice := c.Query("max_price"); maxPrice != "" {
		req.MaxPrice, _ = strconv.ParseFloat(maxPrice, 64)
	}

	resp, err := h.Book.SearchBooks(c.Request.Context(), req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.SuccessWithPage(c, resp.Books, resp.Total, page, pageSize)
}

func (h *Handlers) GetBookDetailHandler(c *gin.Context) {
	bookID := c.Param("id")
	resp, err := h.Book.GetBookDetail(c.Request.Context(), &bookPb.GetBookDetailRequest{
		BookId: bookID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) ListCategoriesHandler(c *gin.Context) {
	resp, err := h.Book.ListCategories(c.Request.Context(), &bookPb.ListCategoriesRequest{})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp.Categories)
}

func (h *Handlers) CreateBookHandler(c *gin.Context) {
	var req bookPb.CreateBookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.Book.CreateBook(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}
