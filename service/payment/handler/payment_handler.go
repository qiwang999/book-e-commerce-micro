package handler

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	pb "github.com/qiwang/book-e-commerce-micro/proto/payment"
	"github.com/qiwang/book-e-commerce-micro/service/payment/model"
	"github.com/qiwang/book-e-commerce-micro/service/payment/repository"
	"gorm.io/gorm"
)

var allowedPaymentMethods = map[string]bool{
	"simulated": true, "alipay": true, "wechat": true, "credit_card": true,
}

// isNotFound reports whether err indicates a missing row. GORM usually returns
// ErrRecordNotFound, but repository code may wrap it; some drivers only expose text.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "record not found") || strings.Contains(msg, "payment not found")
}

type PaymentHandler struct {
	repo *repository.PaymentRepo
}

func NewPaymentHandler(repo *repository.PaymentRepo) *PaymentHandler {
	return &PaymentHandler{repo: repo}
}

func (h *PaymentHandler) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest, rsp *pb.Payment) error {
	if req.UserId == 0 {
		return errors.New("user_id is required")
	}
	if req.OrderId == 0 {
		return errors.New("order_id is required")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be positive")
	}

	method := req.Method
	if method == "" {
		method = "simulated"
	}
	if !allowedPaymentMethods[method] {
		return errors.New("unsupported payment method")
	}

	p := &model.Payment{
		PaymentNo: util.GeneratePaymentNo(),
		OrderID:   req.OrderId,
		UserID:    req.UserId,
		Amount:    req.Amount,
		Method:    method,
		Status:    "pending",
	}

	if err := h.repo.CreatePayment(ctx, p); err != nil {
		log.Printf("CreatePayment error: %v", err)
		return errors.New("failed to create payment")
	}

	modelToProto(p, rsp)
	return nil
}

func (h *PaymentHandler) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest, rsp *pb.PaymentResult) error {
	if req.PaymentNo == "" {
		rsp.Code = 400
		rsp.Message = "payment_no is required"
		return nil
	}

	p, err := h.repo.ProcessPayment(ctx, req.PaymentNo)
	if err != nil {
		log.Printf("ProcessPayment error: %v", err)
		if isNotFound(err) {
			rsp.Code = 404
			rsp.Message = "payment not found"
			return nil
		}
		if strings.Contains(err.Error(), "already processed") {
			rsp.Code = 400
			rsp.Message = err.Error()
			return nil
		}
		rsp.Code = 500
		rsp.Message = "failed to process payment"
		return nil
	}

	rsp.Code = 0
	rsp.Message = "payment processed successfully"
	rsp.Payment = &pb.Payment{}
	modelToProto(p, rsp.Payment)
	return nil
}

func (h *PaymentHandler) GetPaymentStatus(ctx context.Context, req *pb.GetPaymentStatusRequest, rsp *pb.Payment) error {
	if req.PaymentNo == "" {
		return errors.New("payment_no is required")
	}
	p, err := h.repo.GetByPaymentNo(ctx, req.PaymentNo)
	if err != nil {
		if isNotFound(err) {
			return errors.New("payment not found")
		}
		log.Printf("GetPaymentStatus error: %v", err)
		return err
	}

	modelToProto(p, rsp)
	return nil
}

func (h *PaymentHandler) RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest, rsp *pb.PaymentResult) error {
	if req.PaymentNo == "" {
		rsp.Code = 400
		rsp.Message = "payment_no is required"
		return nil
	}

	p, err := h.repo.RefundPayment(ctx, req.PaymentNo)
	if err != nil {
		log.Printf("RefundPayment error: %v", err)
		if isNotFound(err) {
			rsp.Code = 404
			rsp.Message = "payment not found"
			return nil
		}
		if strings.Contains(err.Error(), "only successful payments") {
			rsp.Code = 400
			rsp.Message = err.Error()
			return nil
		}
		rsp.Code = 500
		rsp.Message = "failed to refund payment"
		return nil
	}

	rsp.Code = 0
	rsp.Message = "payment refunded successfully"
	rsp.Payment = &pb.Payment{}
	modelToProto(p, rsp.Payment)
	return nil
}

func (h *PaymentHandler) GetPaymentByOrderId(ctx context.Context, req *pb.GetPaymentByOrderIdRequest, rsp *pb.Payment) error {
	if req.OrderId == 0 {
		return errors.New("order_id is required")
	}
	p, err := h.repo.GetByOrderID(ctx, req.OrderId)
	if err != nil {
		if isNotFound(err) {
			return errors.New("payment not found for this order")
		}
		log.Printf("GetPaymentByOrderId error: %v", err)
		return err
	}

	modelToProto(p, rsp)
	return nil
}

func modelToProto(m *model.Payment, p *pb.Payment) {
	p.Id = m.ID
	p.PaymentNo = m.PaymentNo
	p.OrderId = m.OrderID
	p.UserId = m.UserID
	p.Amount = m.Amount
	p.Method = m.Method
	p.Status = m.Status
	p.CreatedAt = m.CreatedAt.Format(time.RFC3339)
	if m.PaidAt != nil {
		p.PaidAt = m.PaidAt.Format(time.RFC3339)
	}
	if m.RefundedAt != nil {
		p.RefundedAt = m.RefundedAt.Format(time.RFC3339)
	}
}
