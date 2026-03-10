package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const intentClassifyTimeout = 3 * time.Second

// IntentRouter classifies user messages into ToolGroups. It uses fast keyword
// matching as the primary strategy, with an optional LLM fallback when keywords
// alone only match the discovery group — preventing missed intent detection for
// implicit phrases like "帮我把刚才那几本都要了".
//
// All keywords and LLM prompt content are derived dynamically from the
// ToolRegistry's GroupMeta — nothing is hardcoded here.
type IntentRouter struct {
	registry      *ToolRegistry
	classifyModel model.ToolCallingChatModel
}

// NewIntentRouter creates an IntentRouter. Pass a ToolRegistry for keyword and
// prompt data, and an optional ChatModel to enable LLM fallback. Pass nil for
// classifyModel to use keyword-only mode. Pass nil for registry to disable
// all classification (will always return all groups).
func NewIntentRouter(registry *ToolRegistry, classifyModel model.ToolCallingChatModel) *IntentRouter {
	return &IntentRouter{
		registry:      registry,
		classifyModel: classifyModel,
	}
}

// Classify returns the set of ToolGroups relevant to the current message.
// It first tries keyword matching using GroupMeta.Keywords from the registry.
// If only the discovery group is matched and an LLM classifyModel is available,
// it calls the LLM as a fallback.
func (r *IntentRouter) Classify(ctx context.Context, message string, history []string) []ToolGroup {
	if r.registry == nil {
		return r.allGroups()
	}

	groups := map[ToolGroup]bool{
		GroupDiscovery: true,
	}

	check := func(text string) {
		lower := strings.ToLower(text)
		for _, g := range r.registry.Groups() {
			if g == GroupDiscovery {
				continue
			}
			meta := r.registry.GetGroupMeta(g)
			if meta != nil && containsAny(lower, meta.Keywords) {
				groups[g] = true
				if g == GroupOrder {
					groups[GroupShopping] = true
				}
			}
		}
	}

	check(message)

	tail := history
	if len(tail) > 2 {
		tail = tail[len(tail)-2:]
	}
	for _, h := range tail {
		check(h)
	}

	if len(groups) == 1 && r.classifyModel != nil {
		llmGroups := r.llmClassify(ctx, message)
		for _, g := range llmGroups {
			groups[g] = true
		}
	}

	return groupMapToSlice(groups)
}

// llmClassify sends a dynamically built classification prompt to the LLM.
// On any error or timeout it returns all groups as a safe fallback.
func (r *IntentRouter) llmClassify(ctx context.Context, message string) []ToolGroup {
	ctx, cancel := context.WithTimeout(ctx, intentClassifyTimeout)
	defer cancel()

	prompt := r.buildClassifyPrompt(message)
	resp, err := r.classifyModel.Generate(ctx, []*schema.Message{
		{Role: schema.User, Content: prompt},
	})
	if err != nil {
		log.Printf("[IntentRouter] LLM classify error, falling back to all groups: %v", err)
		return r.allGroups()
	}

	return r.parseIntentResponse(resp.Content)
}

// buildClassifyPrompt dynamically constructs the intent classification prompt
// from the registry's group metadata — no hardcoded group names or descriptions.
func (r *IntentRouter) buildClassifyPrompt(message string) string {
	var b strings.Builder
	b.WriteString("将以下用户消息分类到一个或多个意图类别。只输出类别名，用逗号分隔，不要输出任何其他内容。\n\n可选类别：\n")
	for _, g := range r.registry.Groups() {
		meta := r.registry.GetGroupMeta(g)
		hint := string(g)
		if meta != nil && meta.IntentHint != "" {
			hint = meta.IntentHint
		}
		b.WriteString(fmt.Sprintf("- %s（%s）\n", string(g), hint))
	}
	b.WriteString(fmt.Sprintf("\n消息：%s", message))
	return b.String()
}

// parseIntentResponse parses comma-separated group names from LLM output,
// validating each against the registry's registered groups.
func (r *IntentRouter) parseIntentResponse(content string) []ToolGroup {
	content = strings.TrimSpace(strings.ToLower(content))
	if content == "" {
		return r.allGroups()
	}

	registered := make(map[string]ToolGroup)
	for _, g := range r.registry.Groups() {
		registered[string(g)] = g
	}

	var result []ToolGroup
	for _, part := range strings.Split(content, ",") {
		part = strings.TrimSpace(part)
		if g, ok := registered[part]; ok {
			result = append(result, g)
		}
	}

	if len(result) == 0 {
		return r.allGroups()
	}
	return result
}

// allGroups returns every registered group as a safe fallback.
func (r *IntentRouter) allGroups() []ToolGroup {
	return r.registry.Groups()
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func groupMapToSlice(m map[ToolGroup]bool) []ToolGroup {
	groups := make([]ToolGroup, 0, len(m))
	for g := range m {
		groups = append(groups, g)
	}
	return groups
}
