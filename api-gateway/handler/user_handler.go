package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	userPb "github.com/qiwang/book-e-commerce-micro/proto/user"
)

func (h *Handlers) RegisterHandler(c *gin.Context) {
	var req userPb.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.User.Register(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) LoginHandler(c *gin.Context) {
	var req userPb.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}

	resp, err := h.User.Login(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetProfileHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	resp, err := h.User.GetProfile(c.Request.Context(), &userPb.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) UpdateProfileHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req userPb.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	resp, err := h.User.UpdateProfile(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetUserPreferencesHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	resp, err := h.User.GetUserPreferences(c.Request.Context(), &userPb.GetPreferencesRequest{
		UserId: userID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) UpdateUserPreferencesHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req userPb.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, "invalid request body")
		return
	}
	req.UserId = userID

	resp, err := h.User.UpdateUserPreferences(c.Request.Context(), &req)
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) ListAddressesHandler(c *gin.Context) {
	userID := c.GetUint64("user_id")
	resp, err := h.User.ListAddresses(c.Request.Context(), &userPb.ListAddressesRequest{UserId: userID})
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.Success(c, resp.Addresses)
}

func (h *Handlers) CreateAddressHandler(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Phone     string `json:"phone" binding:"required"`
		Province  string `json:"province" binding:"required"`
		City      string `json:"city" binding:"required"`
		District  string `json:"district" binding:"required"`
		Detail    string `json:"detail" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.BadRequest(c, err.Error())
		return
	}
	userID := c.GetUint64("user_id")
	resp, err := h.User.CreateAddress(c.Request.Context(), &userPb.CreateAddressRequest{
		UserId: userID, Name: req.Name, Phone: req.Phone,
		Province: req.Province, City: req.City, District: req.District,
		Detail: req.Detail, IsDefault: req.IsDefault,
	})
	if err != nil {
		util.InternalError(c, err.Error())
		return
	}
	util.Success(c, resp)
}
