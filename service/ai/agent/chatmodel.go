package agent

import (
	"context"
	"fmt"

	einoOpenAI "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/qiwang/book-e-commerce-micro/common/config"
)

func NewChatModel(ctx context.Context, cfg *config.OpenAIConfig) (model.ToolCallingChatModel, error) {
	return NewChatModelWithTemperature(ctx, cfg, 0.7)
}

func NewChatModelWithTemperature(ctx context.Context, cfg *config.OpenAIConfig, temperature float32) (model.ToolCallingChatModel, error) {
	mdl := cfg.Model
	if mdl == "" {
		mdl = "deepseek-v3.2"
	}

	opts := &einoOpenAI.ChatModelConfig{
		APIKey:      cfg.APIKey,
		Model:       mdl,
		Temperature: &temperature,
	}
	if cfg.BaseURL != "" {
		opts.BaseURL = cfg.BaseURL
	}

	cm, err := einoOpenAI.NewChatModel(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("create eino openai chatmodel (temp=%.1f): %w", temperature, err)
	}
	return cm, nil
}
