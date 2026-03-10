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

## Output Format
Return a JSON object with fields:
  title (string): the book title
  summary (string): 2-3 paragraph professional summary covering plot/content, writing style, and what makes it noteworthy
  key_themes (array of strings): 3-6 central themes
  target_audience (string): who would enjoy this book most
  reading_difficulty (string): one of "Beginner", "Intermediate", "Advanced"
  estimated_reading_hours (integer): rough estimate

Only return the JSON object, no extra text, no markdown fences.`

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
