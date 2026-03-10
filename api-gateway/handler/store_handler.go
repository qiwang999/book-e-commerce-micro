package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/qiwang/book-e-commerce-micro/common/util"
	storePb "github.com/qiwang/book-e-commerce-micro/proto/store"
)

func (h *Handlers) ListStoresHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	resp, err := h.Store.ListStores(c.Request.Context(), &storePb.ListStoresRequest{
		City:     c.Query("city"),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.SuccessWithPage(c, resp.Stores, resp.Total, page, pageSize)
}

func (h *Handlers) GetStoreDetailHandler(c *gin.Context) {
	storeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.BadRequest(c, "invalid store id")
		return
	}

	resp, err := h.Store.GetStoreDetail(c.Request.Context(), &storePb.GetStoreDetailRequest{
		StoreId: storeID,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetNearestStoreHandler(c *gin.Context) {
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		util.BadRequest(c, "invalid latitude")
		return
	}
	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil || lng < -180 || lng > 180 {
		util.BadRequest(c, "invalid longitude")
		return
	}

	resp, err := h.Store.GetNearestStore(c.Request.Context(), &storePb.GetNearestStoreRequest{
		Latitude:  lat,
		Longitude: lng,
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}

func (h *Handlers) GetStoresInRadiusHandler(c *gin.Context) {
	lat, err := strconv.ParseFloat(c.Query("lat"), 64)
	if err != nil || lat < -90 || lat > 90 {
		util.BadRequest(c, "invalid latitude")
		return
	}
	lng, err := strconv.ParseFloat(c.Query("lng"), 64)
	if err != nil || lng < -180 || lng > 180 {
		util.BadRequest(c, "invalid longitude")
		return
	}
	radius, err := strconv.ParseFloat(c.Query("radius"), 64)
	if err != nil || radius <= 0 || radius > 500 {
		util.BadRequest(c, "invalid radius (must be 0-500 km)")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	resp, err := h.Store.GetStoresInRadius(c.Request.Context(), &storePb.GetStoresInRadiusRequest{
		Latitude:  lat,
		Longitude: lng,
		RadiusKm:  radius,
		Limit:     int32(limit),
	})
	if err != nil {
		util.InternalError(c, "service unavailable")
		return
	}
	util.Success(c, resp)
}
