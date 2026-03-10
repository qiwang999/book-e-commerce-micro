package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	orderPb "github.com/qiwang/book-e-commerce-micro/proto/order"
)

func (h *Handlers) CreateOrderHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req orderPb.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	resp, err := h.Order.CreateOrder(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) ListOrdersHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	resp, err := h.Order.ListOrders(c.Request.Context(), &orderPb.ListOrdersRequest{
		UserId:   userID,
		Status:   c.Query("status"),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.SuccessWithPage(c, resp.Orders, resp.Total, page, pageSize)
}

func (h *Handlers) GetOrderHandler(c *gin.Context) {
	orderID := c.Param("id")
	userID := c.GetUint64("user_id")

	req := &orderPb.GetOrderRequest{UserId: userID}
	if id, err := strconv.ParseUint(orderID, 10, 64); err == nil {
		req.OrderId = id
	} else {
		req.OrderNo = orderID
	}

	resp, err := h.Order.GetOrder(c.Request.Context(), req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) CancelOrderHandler(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "invalid order id")
		return
	}
	userID := c.GetUint64("user_id")
	resp, err := h.Order.CancelOrder(c.Request.Context(), &orderPb.CancelOrderRequest{
		OrderId: orderID, UserId: userID,
	})
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	if resp.Code != 0 {
		util.Error(c, http.StatusBadRequest, int(resp.Code), resp.Message)
		return
	}
	util.Success(c, nil)
}
