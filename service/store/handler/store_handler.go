package handler

import (
	"context"
	"fmt"

	pb "github.com/qiwang/book-e-commerce-micro/proto/store"
	"github.com/qiwang/book-e-commerce-micro/service/store/model"
	"github.com/qiwang/book-e-commerce-micro/service/store/repository"
)

type StoreHandler struct {
	repo *repository.StoreRepository
}

func NewStoreHandler(repo *repository.StoreRepository) *StoreHandler {
	return &StoreHandler{repo: repo}
}

func (h *StoreHandler) ListStores(ctx context.Context, req *pb.ListStoresRequest, rsp *pb.StoreListResponse) error {
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	stores, total, err := h.repo.FindByCity(ctx, req.City, offset, pageSize)
	if err != nil {
		return err
	}

	rsp.Total = total
	rsp.Stores = make([]*pb.Store, 0, len(stores))
	for _, s := range stores {
		rsp.Stores = append(rsp.Stores, modelToProto(s, 0))
	}
	return nil
}

func (h *StoreHandler) GetStoreDetail(ctx context.Context, req *pb.GetStoreDetailRequest, rsp *pb.Store) error {
	s, err := h.repo.FindByID(ctx, req.StoreId)
	if err != nil {
		return err
	}
	copyToProto(s, rsp, 0)
	return nil
}

func (h *StoreHandler) GetNearestStore(ctx context.Context, req *pb.GetNearestStoreRequest, rsp *pb.Store) error {
	result, err := h.repo.FindNearestStore(ctx, req.Latitude, req.Longitude)
	if err != nil {
		return err
	}
	copyToProto(&result.Store, rsp, result.Distance)
	return nil
}

func (h *StoreHandler) GetStoresInRadius(ctx context.Context, req *pb.GetStoresInRadiusRequest, rsp *pb.StoreListResponse) error {
	results, err := h.repo.FindStoresInRadius(ctx, req.Latitude, req.Longitude, req.RadiusKm, int(req.Limit))
	if err != nil {
		return err
	}

	rsp.Total = int64(len(results))
	rsp.Stores = make([]*pb.Store, 0, len(results))
	for _, r := range results {
		rsp.Stores = append(rsp.Stores, modelToProto(&r.Store, r.Distance))
	}
	return nil
}

func (h *StoreHandler) CreateStore(ctx context.Context, req *pb.CreateStoreRequest, rsp *pb.Store) error {
	if req.Name == "" || req.Address == "" || req.City == "" {
		return fmt.Errorf("name, address and city are required")
	}
	if req.Latitude < -90 || req.Latitude > 90 || req.Longitude < -180 || req.Longitude > 180 {
		return fmt.Errorf("invalid latitude or longitude")
	}

	s := &model.Store{
		Name:          req.Name,
		Description:   req.Description,
		Address:       req.Address,
		City:          req.City,
		District:      req.District,
		Phone:         req.Phone,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		BusinessHours: req.BusinessHours,
		Status:        1,
	}

	if err := h.repo.Create(ctx, s); err != nil {
		return err
	}

	h.repo.CacheStoreCoords(ctx, s.ID, s.Latitude, s.Longitude)
	copyToProto(s, rsp, 0)
	return nil
}

func (h *StoreHandler) UpdateStore(ctx context.Context, req *pb.UpdateStoreRequest, rsp *pb.Store) error {
	s, err := h.repo.FindByID(ctx, req.Id)
	if err != nil {
		return err
	}

	if req.Name != "" {
		s.Name = req.Name
	}
	if req.Description != "" {
		s.Description = req.Description
	}
	if req.Address != "" {
		s.Address = req.Address
	}
	if req.Phone != "" {
		s.Phone = req.Phone
	}
	if req.BusinessHours != "" {
		s.BusinessHours = req.BusinessHours
	}
	if req.Status != 0 {
		s.Status = req.Status
	}

	if err := h.repo.Update(ctx, s); err != nil {
		return err
	}

	copyToProto(s, rsp, 0)
	return nil
}

func modelToProto(s *model.Store, distanceKm float64) *pb.Store {
	return &pb.Store{
		Id:            s.ID,
		Name:          s.Name,
		Description:   s.Description,
		Address:       s.Address,
		City:          s.City,
		District:      s.District,
		Phone:         s.Phone,
		Latitude:      s.Latitude,
		Longitude:     s.Longitude,
		BusinessHours: s.BusinessHours,
		Status:        s.Status,
		ImageUrl:      s.ImageURL,
		DistanceKm:    distanceKm,
	}
}

func copyToProto(s *model.Store, rsp *pb.Store, distanceKm float64) {
	rsp.Id = s.ID
	rsp.Name = s.Name
	rsp.Description = s.Description
	rsp.Address = s.Address
	rsp.City = s.City
	rsp.District = s.District
	rsp.Phone = s.Phone
	rsp.Latitude = s.Latitude
	rsp.Longitude = s.Longitude
	rsp.BusinessHours = s.BusinessHours
	rsp.Status = s.Status
	rsp.ImageUrl = s.ImageURL
	rsp.DistanceKm = distanceKm
}
