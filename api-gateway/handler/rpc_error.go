package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
)

// respondOrderRPCError maps order-service RPC errors to HTTP responses.
// Returns true if the error was handled (response already written).
func respondOrderRPCError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "order not found"):
		util.NotFound(c, err.Error())
		return true
	case strings.Contains(msg, "insufficient stock"),
		strings.Contains(msg, "is required"),
		strings.Contains(msg, "must have at least one"),
		strings.Contains(msg, "positive quantity and price"),
		strings.Contains(msg, "positive quantity"),
		strings.Contains(msg, "amount must be positive"):
		util.BadRequest(c, err.Error())
		return true
	default:
		return false
	}
}

// respondPaymentRPCError maps payment-service RPC errors to HTTP responses.
func respondPaymentRPCError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "payment not found"),
		strings.Contains(msg, "record not found"):
		util.NotFound(c, err.Error())
		return true
	case strings.Contains(msg, "is required"),
		strings.Contains(msg, "unsupported payment"),
		strings.Contains(msg, "must be positive"):
		util.BadRequest(c, err.Error())
		return true
	default:
		return false
	}
}
