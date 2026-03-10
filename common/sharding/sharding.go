package sharding

import (
	"fmt"
	"hash/fnv"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ShardConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

func (c ShardConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

// ShardRouter manages multiple database shards and routes queries
// based on a shard key. It uses consistent hashing (FNV-1a) to map
// uint64 keys to shard indices.
type ShardRouter struct {
	dbs    []*gorm.DB
	shards int
}

// NewShardRouter creates connections to all configured shards.
// It also runs AutoMigrate for the given models on every shard.
func NewShardRouter(configs []ShardConfig, models ...interface{}) (*ShardRouter, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("sharding: at least one shard config is required")
	}

	dbs := make([]*gorm.DB, len(configs))
	for i, cfg := range configs {
		db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("sharding: connect shard %d (%s): %w", i, cfg.Database, err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("sharding: get sql.DB for shard %d: %w", i, err)
		}
		maxOpen := cfg.MaxOpenConns
		if maxOpen == 0 {
			maxOpen = 50
		}
		maxIdle := cfg.MaxIdleConns
		if maxIdle == 0 {
			maxIdle = 10
		}
		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetMaxIdleConns(maxIdle)

		if len(models) > 0 {
			if err := db.AutoMigrate(models...); err != nil {
				return nil, fmt.Errorf("sharding: migrate shard %d: %w", i, err)
			}
		}

		log.Printf("[sharding] shard %d connected: %s", i, cfg.Database)
		dbs[i] = db
	}

	return &ShardRouter{dbs: dbs, shards: len(dbs)}, nil
}

// GetDB returns the gorm.DB for the shard that owns the given key.
func (r *ShardRouter) GetDB(shardKey uint64) *gorm.DB {
	idx := shardKey % uint64(r.shards)
	return r.dbs[idx]
}

// GetDBByHash hashes a string key (e.g. order_no) then routes to a shard.
func (r *ShardRouter) GetDBByHash(key string) *gorm.DB {
	h := fnv.New64a()
	h.Write([]byte(key))
	return r.GetDB(h.Sum64())
}

// ShardCount returns the number of configured shards.
func (r *ShardRouter) ShardCount() int {
	return r.shards
}

// AllDBs returns all shard connections, useful for fan-out queries
// like admin listing across all shards.
func (r *ShardRouter) AllDBs() []*gorm.DB {
	return r.dbs
}

// ShardIndex returns which shard index a key maps to.
func (r *ShardRouter) ShardIndex(shardKey uint64) int {
	return int(shardKey % uint64(r.shards))
}
