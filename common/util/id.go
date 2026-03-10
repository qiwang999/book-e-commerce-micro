package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.New().String()
}

func cryptoRandN(n int64) int64 {
	val, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return time.Now().UnixNano() % n
	}
	return val.Int64()
}

// GenerateOrderNo creates an order number that embeds the user_id shard info.
// Format: BH<2-digit shard key><yyyyMMddHHmmss><6-digit micro><4-digit random>
// The shard key (userID % 100) allows extracting the shard from order_no alone.
func GenerateOrderNo(userID uint64) string {
	shardKey := userID % 100
	now := time.Now()
	return fmt.Sprintf("BH%02d%s%06d%04d",
		shardKey,
		now.Format("20060102150405"),
		now.Nanosecond()/1000,
		cryptoRandN(10000))
}

// ExtractShardKeyFromOrderNo parses the embedded shard key from an order number.
// Returns the raw shard key (userID % 100), which can be used with ShardRouter.
func ExtractShardKeyFromOrderNo(orderNo string) (uint64, error) {
	if !strings.HasPrefix(orderNo, "BH") || len(orderNo) < 4 {
		return 0, fmt.Errorf("invalid order_no format: %s", orderNo)
	}
	key, err := strconv.ParseUint(orderNo[2:4], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse shard key from order_no %s: %w", orderNo, err)
	}
	return key, nil
}

func GeneratePaymentNo() string {
	now := time.Now()
	return fmt.Sprintf("PAY%s%06d%04d",
		now.Format("20060102150405"),
		now.Nanosecond()/1000,
		cryptoRandN(10000))
}
