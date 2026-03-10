package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	inventoryPb "github.com/qiwang/book-e-commerce-micro/proto/inventory"
)

func (h *Handlers) CheckStockHandler(c *gin.Context) {
	storeID, err := strconv.ParseUint(c.Query("store_id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "invalid store_id")
		return
	}
	bookID := c.Query("book_id")
	if bookID == "" {
		util.BadRequest(c, "book_id required")
		return
	}
	resp, err := h.Inventory.CheckStock(c.Request.Context(), &inventoryPb.CheckStockRequest{
		StoreId: storeID, BookId: bookID,
	})
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetStoreBooksHandler(c *gin.Context) {
	storeID, err := strconv.ParseUint(c.Param("store_id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "invalid store_id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.Inventory.GetStoreBooks(c.Request.Context(), &inventoryPb.GetStoreBooksRequest{
		StoreId: storeID, Page: int32(page), PageSize: int32(pageSize),
	})
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.Success(c, resp)
}
