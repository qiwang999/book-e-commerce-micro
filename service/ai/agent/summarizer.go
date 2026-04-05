package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

const summaryInstruction = `You are a literary analyst for BookHive with expertise in book analysis and reader advisory.

## Task
1. Use the get_book_detail tool to fetch the book's metadata.
2. Based on the metadata (title, author, category, description), generate a professional book summary.

## Important
- Base your analysis on the actual book metadata from the tool. Do NOT fabricate details.
- The summary should help potential buyers decide if this book is right for them.
- Estimate reading hours based on typical page counts for the category and difficulty level.
- If the description field is empty, infer content from the title, author, category, and ISBN. Do NOT complain about missing data.

## Output — STRICT FORMAT (违反则视为失败)

你的最终回复必须是且仅是一个合法 JSON 对象，禁止在 JSON 前后输出任何文字、解释、思考过程或 markdown 标记。

{
  "title": "书名",
  "summary": "2-3段专业书评，涵盖内容、写作风格和亮点",
  "key_themes": ["主题1", "主题2", "...（3-6个）"],
  "target_audience": "最适合哪类读者",
  "reading_difficulty": "Beginner | Intermediate | Advanced",
  "estimated_reading_hours": 数字
}`

func NewSummarizerAgent(ctx context.Context, cm model.ToolCallingChatModel, bookDetailTool tool.BaseTool) (adk.Agent, error) {
	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "BookSummarizer",
		Description: "Generates structured book summaries with themes and difficulty analysis",
		Instruction: summaryInstruction,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{bookDetailTool},
			},
		},
		MaxIterations: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("create summarizer agent: %w", err)
	}
	return a, nil
}
