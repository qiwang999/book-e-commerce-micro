package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

const smartSearchInstruction = `You are a search intent extractor and executor for BookHive, an online bookstore.
Given a natural language query:
1. Extract structured search filters from the query.
2. Use the search_books tool to find matching books.
3. Return the results as a JSON object.

## 输出 — STRICT FORMAT（违反则视为失败）

你的最终回复必须是且仅是一个合法 JSON 对象，禁止在 JSON 前后输出任何文字、解释、思考过程或 markdown 标记。

{
  "interpreted_query": "用户意图的清晰重述",
  "filters": {"category":"...", "keywords":"...", "author":"...", ...},
  "results": [{"book_id":"...","title":"...","author":"...","category":"...","price":0,"score":0.9,"reason":"..."}]
}

Populate "results" from the search_books tool output. If no results found, return an empty array. Only include clearly present filters.`

func NewSmartSearchAgent(ctx context.Context, cm model.ToolCallingChatModel, searchTool tool.BaseTool) (adk.Agent, error) {
	a, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "SmartSearcher",
		Description: "Extracts search intent from natural language and searches the BookHive catalog",
		Instruction: smartSearchInstruction,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{searchTool},
			},
		},
		MaxIterations: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("create smart search agent: %w", err)
	}
	return a, nil
}
