package provider

import (
	"context"

	"gollm-mini/internal/types"
)

type Provider interface {
	//Generate 核心能力：把若干消息发给模型，返回一段文本
	Generate(ctx context.Context, messages []types.Message) (text string, usage types.Usage, err error)

	// Stream 可选实现；未实现时由 core 层降级到 Generate
	Stream(ctx context.Context, messages []types.Message, cb func(types.Chunk)) (usage types.Usage, err error)
}

type ModelSetter interface {
	SetModel(string)
}
