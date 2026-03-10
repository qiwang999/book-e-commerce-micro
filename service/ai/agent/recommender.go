package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
)

const recommendInstruction = `You are a professional book recommendation engine for BookHive, an online bookstore.

## 核心原则（CRITICAL — 必须遵守）

1. **仅推荐书库中存在的书**：如果用户的消息中附带了「书库检索结果」，你只能从检索结果中选择书籍进行推荐，绝不要编造不存在的书。
2. **库存不足必须标注**：如果检索结果中某本书标注"库存不足"，你必须在 reason 字段中注明"（⚠️ 当前库存不足）"。
3. **有货的书优先推荐**：在多本书中选择推荐时，优先推荐有货的书籍，将库存不足的书排在后面。
4. **如果没有检索结果**：说明暂无匹配的书籍可推荐，不要凭空编造。
5. **多样性**：推荐结果应尽量涵盖不同分类（category），避免所有推荐都来自同一分类。至少包含2个不同分类。
6. **个性化**：如果提供了用户的购买历史，避免推荐用户已购买过的书；推荐与其阅读偏好互补的书籍，既有熟悉领域的深入探索，也有可能感兴趣的新领域。

## 输出格式

Return a JSON array of objects with fields: book_id, title, author, category, score (0-1), reason.
Only return the JSON array, no extra text, no markdown fences.`

func NewRecommenderAgent(ctx context.Context, cm model.ToolCallingChatModel) (adk.Agent, error) {
	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "BookRecommender",
		Description: "Generates personalized book recommendations based on user context",
		Instruction: recommendInstruction,
		Model:       cm,
	})
	if err != nil {
		return nil, fmt.Errorf("create recommender agent: %w", err)
	}
	return a, nil
}
