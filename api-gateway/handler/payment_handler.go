package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	paymentPb "github.com/qiwang/book-e-commerce-micro/proto/payment"
)

func (h *Handlers) CreatePaymentHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req paymentPb.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	if req.OrderId == 0 {
		util.BadRequest(c, "order_id is required")
		return
	}
	if req.Amount <= 0 {
		util.BadRequest(c, "amount must be positive")
		return
	}

	resp, err := h.Payment.CreatePayment(c.Request.Context(), &req)
	if err != nil {
		if respondPaymentRPCError(c, err) {
			return
		}
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) ProcessPaymentHandler(c *gin.Context) {
	paymentNo := c.Param("payment_no")

	resp, err := h.Payment.ProcessPayment(c.Request.Context(), &paymentPb.ProcessPaymentRequest{
		PaymentNo: paymentNo,
	})
	if err != nil {
		if respondPaymentRPCError(c, err) {
			return
		}
		util.InternalError(c, "service unavailable")
		return
	}
	if resp.GetCode() != 0 {
		httpSt := http.StatusBadRequest
		switch resp.GetCode() {
		case 404:
			httpSt = http.StatusNotFound
		case 403:
			httpSt = http.StatusForbidden
		case 500, 502, 503:
			httpSt = http.StatusInternalServerError
		}
		util.Error(c, httpSt, int(resp.GetCode()), resp.GetMessage())
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetPaymentStatusHandler(c *gin.Context) {
	paymentNo := c.Param("payment_no")

	resp, err := h.Payment.GetPaymentStatus(c.Request.Context(), &paymentPb.GetPaymentStatusRequest{
		PaymentNo: paymentNo,
	})
	if err != nil {
		if respondPaymentRPCError(c, err) {
			return
		}
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) RefundPaymentHandler(c *gin.Context) {
	paymentNo := c.Param("payment_no")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	resp, err := h.Payment.RefundPayment(c.Request.Context(), &paymentPb.RefundPaymentRequest{
		PaymentNo: paymentNo, Reason: req.Reason,
	})
	if err != nil {
		if respondPaymentRPCError(c, err) {
			return
		}
		util.InternalError(c, "service unavailable")
		return
	}
	if resp.GetCode() != 0 {
		httpSt := http.StatusBadRequest
		switch resp.GetCode() {
		case 404:
			httpSt = http.StatusNotFound
		case 403:
			httpSt = http.StatusForbidden
		case 500, 502, 503:
			httpSt = http.StatusInternalServerError
		}
		util.Error(c, httpSt, int(resp.GetCode()), resp.GetMessage())
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetPaymentByOrderHandler(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "invalid order id")
		return
	}
	resp, err := h.Payment.GetPaymentByOrderId(c.Request.Context(), &paymentPb.GetPaymentByOrderIdRequest{OrderId: orderID})
	if err != nil {
		if respondPaymentRPCError(c, err) {
			return
		}
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}
