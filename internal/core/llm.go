package core

import (
	"context"
	"gollm-mini/internal/helper"
	"log"
	"time"

	"gollm-mini/internal/provider"
	"gollm-mini/internal/types"
)

const (
	maxCtx = 3000
)

type LLM struct {
	name string
	p    provider.Provider
}

// New 创建一个 LLM 实例并注入模型名（若 Provider 支持）
func New(providerName, model string) (*LLM, error) {
	p, err := provider.Get(providerName)
	if err != nil {
		return nil, err
	}
	if ms, ok := p.(provider.ModelSetter); ok && model != "" {
		ms.SetModel(model)
	}
	return &LLM{name: providerName, p: p}, nil
}

// Generate 调用底层 Provider 的生成接口，并打印日志
func (l *LLM) Generate(ctx context.Context, messages []types.Message) (string, types.Usage, error) {
	//Memory截断
	clipped := helper.TruncateMessages(messages, maxCtx)

	start := time.Now()
	var (
		txt   string
		usage types.Usage
		err   error
	)

	err = Retry(ctx, 3, 300*time.Millisecond, func() error {
		var e error
		txt, usage, e = l.p.Generate(ctx, clipped)
		return e
	})
	dur := time.Since(start)

	log.Printf("[LLM] provider=%s prompt=%d completion=%d total=%d latency=%s",
		l.name, usage.PromptTokens, usage.CompletionTokens, usage.Total(), dur)
	return txt, usage, err
}

// Stream 调用底层 Provider 的流式接口（若实现）
func (l *LLM) Stream(ctx context.Context, messages []types.Message, cb func(types.Chunk)) (types.Usage, error) {
	//Memory截断
	clipped := helper.TruncateMessages(messages, maxCtx)

	ps, streamed := l.p.(interface {
		Stream(context.Context, []types.Message, func(types.Chunk)) (types.Usage, error)
	})

	var (
		usage types.Usage
		err   error
	)
	// 若 Provider 不支持流式，降级为一次性调用

	err = Retry(ctx, 3, 300*time.Millisecond, func() error {
		if streamed {
			usage, err = ps.Stream(ctx, messages, cb)
			return err
		}
		var txt string
		txt, usage, err = l.p.Generate(ctx, clipped)
		if err == nil {
			cb(types.Chunk{Content: txt, Delta: usage.CompletionTokens})
		}
		return err
	})

	return usage, err
}
