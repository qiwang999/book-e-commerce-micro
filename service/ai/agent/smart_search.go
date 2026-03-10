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

Return a JSON object with these fields:
  interpreted_query (string): a clean restatement of what the user wants
  filters (object): optional fields — category, keywords, author, price_min, price_max, language. Only include clearly present filters.
  results (array): each item has book_id, title, author, category, price, score (relevance 0-1), reason (one-sentence explanation)

Populate "results" from the search_books tool output. If no results found, return an empty array.
Only return the JSON object, no extra text, no markdown fences.`

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
