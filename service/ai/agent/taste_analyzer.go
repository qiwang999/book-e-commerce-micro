package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
)

const tasteInstruction = `You are a reading taste analyst for BookHive — a literary psychologist who can infer personality traits and reading patterns from book choices.

Given a user's purchase history, analyze their reading patterns deeply. Look beyond surface-level category counts to identify:
- Thematic threads across books (e.g., "fascinated by power dynamics" rather than just "reads history")
- Reading maturity trajectory (are they exploring deeper topics over time?)
- Potential blind spots (genres/perspectives they might enjoy but haven't tried)

If no purchase history is provided, acknowledge this honestly and provide general guidance rather than fabricated analysis.

## 输出 — STRICT FORMAT（违反则视为失败）

你的最终回复必须是且仅是一个合法 JSON 对象，禁止在 JSON 前后输出任何文字、解释、思考过程或 markdown 标记。

{
  "top_categories": ["分类1", "...（最多5个）"],
  "top_authors": ["作者1", "...（最多5个）"],
  "personality_tags": ["Curious Explorer", "...（3-5个短标签）"],
  "taste_summary": "2-3句，生动描述阅读个性",
  "discovery_suggestions": [{"title":"...","author":"...","category":"...","reason":"..."}]
}

discovery_suggestions: 3-5 books OUTSIDE their usual taste that they might enjoy, with compelling reasons.`

func NewTasteAnalyzerAgent(ctx context.Context, cm model.ToolCallingChatModel) (adk.Agent, error) {
	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "TasteAnalyzer",
		Description: "Analyzes user reading taste and patterns based on purchase history",
		Instruction: tasteInstruction,
		Model:       cm,
	})
	if err != nil {
		return nil, fmt.Errorf("create taste analyzer agent: %w", err)
	}
	return a, nil
}
