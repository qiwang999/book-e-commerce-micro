package hitl

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type ctxKey int

const (
	gateKey ctxKey = iota + 1
	metaKey
)

// Gate coordinates human-in-the-loop confirmation for sensitive tools (Redis-backed).
type Gate struct {
	rdb *redis.Client
	mu  sync.Mutex
	// LastPending is set when a tool blocks waiting for user confirmation (same request).
	LastPending *PendingResponse
}

type PendingResponse struct {
	ActionID string
	Secret   string
	Summary  string
	Tool     string
}

type ChatMeta struct {
	UserID    uint64
	SessionID string
}

func WithGate(ctx context.Context, g *Gate) context.Context {
	return context.WithValue(ctx, gateKey, g)
}

func WithChatMeta(ctx context.Context, userID uint64, sessionID string) context.Context {
	return context.WithValue(ctx, metaKey, ChatMeta{UserID: userID, SessionID: sessionID})
}

func GateFromContext(ctx context.Context) *Gate {
	g, _ := ctx.Value(gateKey).(*Gate)
	return g
}

func MetaFromContext(ctx context.Context) (ChatMeta, bool) {
	m, ok := ctx.Value(metaKey).(ChatMeta)
	return m, ok && m.SessionID != ""
}

func NewGate(rdb *redis.Client) *Gate {
	return &Gate{rdb: rdb}
}

// Enabled is true when Redis is available for pending/approved keys.
func (g *Gate) Enabled() bool {
	return g != nil && g.rdb != nil
}

const (
	pendKeyFmt     = "ai:hitl:pend:%s"
	approvedKeyFmt = "ai:hitl:approved:%d:%s:%s"
	pendTTL        = 15 * time.Minute
	approvedTTL    = 3 * time.Minute
)

type pendingRecord struct {
	UserID    uint64 `json:"user_id"`
	SessionID string `json:"session_id"`
	Tool      string `json:"tool"`
	ArgsJSON  string `json:"args_json"`
	Secret    string `json:"secret"`
}

// ApplyConfirmation validates pending action and stages approved args for the next tool invocation.
func ApplyConfirmation(ctx context.Context, rdb *redis.Client, userID uint64, sessionID, actionID, secret string) error {
	if rdb == nil || actionID == "" || secret == "" {
		return fmt.Errorf("hitl: missing redis or credentials")
	}
	key := fmt.Sprintf(pendKeyFmt, actionID)
	raw, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("hitl: unknown or expired action")
	}
	var rec pendingRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return fmt.Errorf("hitl: corrupt pending record")
	}
	if rec.Secret != secret || rec.UserID != userID || rec.SessionID != sessionID {
		return fmt.Errorf("hitl: confirmation mismatch")
	}
	apKey := fmt.Sprintf(approvedKeyFmt, userID, sessionID, rec.Tool)
	if err := rdb.Set(ctx, apKey, rec.ArgsJSON, approvedTTL).Err(); err != nil {
		return err
	}
	_ = rdb.Del(ctx, key).Err()
	return nil
}

func randomSecret() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// TryConsumeApprovedArgs returns stored JSON args if user confirmed; key is deleted.
func (g *Gate) TryConsumeApprovedArgs(ctx context.Context, meta ChatMeta, tool string) ([]byte, bool) {
	if g == nil || g.rdb == nil {
		return nil, false
	}
	apKey := fmt.Sprintf(approvedKeyFmt, meta.UserID, meta.SessionID, tool)
	raw, err := g.rdb.Get(ctx, apKey).Result()
	if err != nil || raw == "" {
		return nil, false
	}
	_ = g.rdb.Del(ctx, apKey).Err()
	return []byte(raw), true
}

// RegisterPending stores args and exposes action_id + secret for the client; sets g.LastPending.
func (g *Gate) RegisterPending(ctx context.Context, meta ChatMeta, tool string, argsJSON []byte, summary string) error {
	if g == nil || g.rdb == nil {
		return fmt.Errorf("hitl: redis not configured")
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return err
	}
	actionID := hex.EncodeToString(idBytes)
	secret := randomSecret()
	rec := pendingRecord{
		UserID:    meta.UserID,
		SessionID: meta.SessionID,
		Tool:      tool,
		ArgsJSON:  string(argsJSON),
		Secret:    secret,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	key := fmt.Sprintf(pendKeyFmt, actionID)
	if err := g.rdb.Set(ctx, key, string(b), pendTTL).Err(); err != nil {
		return err
	}
	g.mu.Lock()
	g.LastPending = &PendingResponse{
		ActionID: actionID,
		Secret:   secret,
		Summary:  summary,
		Tool:     tool,
	}
	g.mu.Unlock()
	return nil
}

func (g *Gate) TakeLastPending() *PendingResponse {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	p := g.LastPending
	g.LastPending = nil
	return p
}

// ShouldRunRecommenderSubAgent returns true when the user message looks like a pure recommendation ask.
func ShouldRunRecommenderSubAgent(message string, matchedGroups []string) bool {
	m := strings.ToLower(strings.TrimSpace(message))
	if m == "" {
		return false
	}
	// If checkout/order tools might be needed, skip sub-agent.
	for _, g := range matchedGroups {
		if g == "order" || g == "shopping" || g == "order_manage" {
			return false
		}
	}
	kws := []string{"推荐", "荐书", "好书", "recommend", "suggestion", "what to read", "读什么"}
	for _, k := range kws {
		if strings.Contains(m, strings.ToLower(k)) {
			return true
		}
	}
	return false
}
