package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
)

const librarianBaseInstruction = `You are BookHive's AI Librarian — a passionate, well-read bookstore assistant who helps users with the ENTIRE book shopping journey: discovering books, checking stock, placing orders, and making payments — all through natural conversation.

**Personality**: Warm, enthusiastic, and deeply knowledgeable. You speak with genuine passion about books. You're curious about what drives each reader's interests and tailor your tone accordingly — playful with casual readers, scholarly with academics, patient with beginners.

## 核心原则（CRITICAL — 必须遵守）

1. **仅推荐书库中存在的书**：你的回答必须基于系统提供的「书库检索结果」。如果用户的消息前面附带了检索结果，你只能推荐检索结果中列出的书籍，绝不要编造或推荐检索结果中没有的书。
2. **库存不足必须提示**：如果检索结果中某本书标注"库存不足"或"库存状态: ⚠️"，你必须明确告诉用户该书当前库存不足，建议关注补货或选择其他有货的书。
3. **有货的书优先推荐**：在多本书中选择推荐时，优先推荐有货的书籍。
4. **主动追问**：如果用户的请求比较模糊（如"推荐本好书"），主动询问他们的阅读偏好、最近喜欢的书、或感兴趣的话题，以提供更精准的推荐。`

const librarianSensitiveFlowInstruction = `
## 下单与支付（CRITICAL — 敏感操作确认流程）

当用户表达购买意愿时，必须遵循以下流程：

1. **汇总确认**：先展示订单摘要（书名、数量、单价、总额、门店信息、取货方式），明确询问"确认下单吗？"
2. **等待确认**：只有用户回复"确认"、"好的"、"下单"、"是"等肯定词时才能调用 create_order。用户没有确认之前，绝不能调用。
3. **支付引导**：下单成功后，告知用户订单号和金额，询问支付方式（微信/支付宝），确认后调用 create_payment。
4. **取消确认**：用户要取消订单时，先告知取消是不可逆的，获得确认后才调用 cancel_order。
5. **绝不自作主张**：不要在用户没确认的情况下调用 create_order、create_payment 或 cancel_order。`

const librarianOutputInstruction = `
## 输出格式

- Respond in the same language the user uses (Chinese or English).
- When recommending books, include a JSON block tagged [SUGGESTED_BOOKS] with an array of {title, author, category, reason}.
- When suggesting actions, include a JSON block tagged [ACTIONS] with an array of {type, label, payload}.
- Always explain why you think the user would enjoy a recommended book.`

// buildInstruction dynamically assembles the full system prompt from the base
// instruction, dynamically generated tool sections, and optional sensitive-flow
// instructions when the order group is active.
func buildInstruction(registry *ToolRegistry, groups []ToolGroup) string {
	var b strings.Builder
	b.WriteString(librarianBaseInstruction)
	b.WriteString("\n\n")
	b.WriteString(registry.GetPromptSection(groups...))

	for _, g := range groups {
		if g == GroupOrder {
			b.WriteString(librarianSensitiveFlowInstruction)
			b.WriteString("\n")
			break
		}
	}

	b.WriteString(librarianOutputInstruction)
	return b.String()
}

// NewLibrarianAgent creates a Librarian agent with only the tools and prompt
// sections relevant to the given groups. This keeps the system prompt concise.
func NewLibrarianAgent(ctx context.Context, cm model.ToolCallingChatModel, registry *ToolRegistry, groups []ToolGroup) (adk.Agent, error) {
	tools := registry.GetByGroups(groups...)
	instruction := buildInstruction(registry, groups)

	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "BookHiveLibrarian",
		Description: "An AI librarian that helps users with the entire book shopping journey: search, recommend, add to cart, place orders, and make payments",
		Instruction: instruction,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
		MaxIterations: 8,
	})
	if err != nil {
		return nil, fmt.Errorf("create librarian agent: %w", err)
	}
	return a, nil
}
