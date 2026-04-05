package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/qiwang/book-e-commerce-micro/service/store/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	storeCoordsCacheKey = "store:coords:%d"
	storeCacheTTL       = 30 * time.Minute
)

type StoreRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewStoreRepository(db *gorm.DB, rdb *redis.Client) *StoreRepository {
	return &StoreRepository{db: db, rdb: rdb}
}

func (r *StoreRepository) FindByCity(ctx context.Context, city string, offset, limit int) ([]*model.Store, int64, error) {
	var stores []*model.Store
	var total int64

	query := r.db.WithContext(ctx).Model(&model.Store{}).Where("status = ?", 1)
	if city != "" {
		query = query.Where("city = ?", city)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Offset(offset).Limit(limit).Order("id DESC").Find(&stores).Error; err != nil {
		return nil, 0, err
	}

	return stores, total, nil
}

func (r *StoreRepository) FindByID(ctx context.Context, id uint64) (*model.Store, error) {
	var s model.Store
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *StoreRepository) Create(ctx context.Context, s *model.Store) error {
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return err
	}
	// Update the POINT column via raw SQL so spatial indexes work.
	return r.db.WithContext(ctx).Exec(
		"UPDATE stores SET location = ST_GeomFromText(CONCAT('POINT(', ?, ' ', ?, ')'), 4326) WHERE id = ?",
		s.Latitude, s.Longitude, s.ID,
	).Error
}

func (r *StoreRepository) Update(ctx context.Context, s *model.Store) error {
	if err := r.db.WithContext(ctx).Save(s).Error; err != nil {
		return err
	}
	r.invalidateCache(ctx, s.ID)
	return nil
}

func (r *StoreRepository) FindNearestStore(ctx context.Context, lat, lng float64) (*model.StoreWithDistance, error) {
	var result model.StoreWithDistance

	sql := `SELECT *, ST_Distance_Sphere(location, ST_GeomFromText(CONCAT('POINT(', ?, ' ', ?, ')'), 4326)) / 1000 AS distance
			FROM stores WHERE status = 1 ORDER BY distance LIMIT 1`

	if err := r.db.WithContext(ctx).Raw(sql, lat, lng).Scan(&result).Error; err != nil {
		return nil, err
	}
	if result.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &result, nil
}

func (r *StoreRepository) FindStoresInRadius(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]*model.StoreWithDistance, error) {
	if limit <= 0 {
		limit = 20
	}

	var results []*model.StoreWithDistance

	sql := `SELECT *, ST_Distance_Sphere(location, ST_GeomFromText(CONCAT('POINT(', ?, ' ', ?, ')'), 4326)) / 1000 AS distance
			FROM stores WHERE status = 1 HAVING distance <= ? ORDER BY distance LIMIT ?`

	if err := r.db.WithContext(ctx).Raw(sql, lat, lng, radiusKm, limit).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// CacheStoreCoords caches a store's coordinates in Redis.
func (r *StoreRepository) CacheStoreCoords(ctx context.Context, id uint64, lat, lng float64) {
	if r.rdb == nil {
		return
	}
	data, _ := json.Marshal(map[string]float64{"lat": lat, "lng": lng})
	r.rdb.Set(ctx, fmt.Sprintf(storeCoordsCacheKey, id), data, storeCacheTTL)
}

// GetCachedCoords retrieves cached coordinates for a store.
func (r *StoreRepository) GetCachedCoords(ctx context.Context, id uint64) (lat, lng float64, ok bool) {
	if r.rdb == nil {
		return 0, 0, false
	}
	val, err := r.rdb.Get(ctx, fmt.Sprintf(storeCoordsCacheKey, id)).Result()
	if err != nil {
		return 0, 0, false
	}
	var coords map[string]float64
	if err := json.Unmarshal([]byte(val), &coords); err != nil {
		return 0, 0, false
	}
	return coords["lat"], coords["lng"], true
}

func (r *StoreRepository) invalidateCache(ctx context.Context, id uint64) {
	if r.rdb == nil {
		return
	}
	r.rdb.Del(ctx, fmt.Sprintf(storeCoordsCacheKey, id))
}
