package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	cartPb "github.com/qiwang/book-e-commerce-micro/proto/cart"
)

func (h *Handlers) GetCartHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	resp, err := h.Cart.GetCart(c.Request.Context(), &cartPb.GetCartRequest{
		UserId: userID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) AddToCartHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req cartPb.AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	resp, err := h.Cart.AddToCart(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) RemoveFromCartHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	itemID := c.Param("item_id")

	resp, err := h.Cart.RemoveFromCart(c.Request.Context(), &cartPb.RemoveFromCartRequest{
		UserId: userID,
		ItemId: itemID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) UpdateCartItemHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	itemID := c.Param("item_id")

	var req cartPb.UpdateCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID
	req.ItemId = itemID

	resp, err := h.Cart.UpdateCartItem(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) ClearCartHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	resp, err := h.Cart.ClearCart(c.Request.Context(), &cartPb.ClearCartRequest{
		UserId: userID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}
