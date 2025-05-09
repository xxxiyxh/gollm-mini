package ollama

import (
	"context"
	"github.com/ollama/ollama/api" // 官方 SDK
	"gollm-mini/internal/provider" // 注册表
	"gollm-mini/internal/types"
)

// Ollama 实现 gollm-mini-mini 的 Provider 接口
type Ollama struct {
	client *api.Client
	model  string
}

func (o *Ollama) SetModel(m string) { o.model = m }

// New 返回一个 Ollama Provider；如果你想连到远端，把 baseURL 写进去
func New(model string) *Ollama {
	cli, _ := api.ClientFromEnvironment() // 读 OLLAMA_HOST，不设就用本地
	return &Ollama{client: cli, model: model}
}

// Generate 把历史对话打给 /api/chat，取最后一条回复
func (o *Ollama) Generate(ctx context.Context, msgs []types.Message) (string, types.Usage, error) {
	om := make([]api.Message, len(msgs))
	for i, m := range msgs {
		om[i] = api.Message{Role: string(m.Role), Content: m.Content}
	}
	stream := false
	req := &api.ChatRequest{
		Model:    o.model,
		Messages: om,
		Stream:   &stream,
	}
	var (
		full  string
		usage types.Usage
	)
	// Chat 会把每个（可能是流）chunk 交给回调
	if err := o.client.Chat(ctx, req, func(cr api.ChatResponse) error {
		full = cr.Message.Content
		usage = types.Usage{
			PromptTokens:     cr.Metrics.PromptEvalCount,
			CompletionTokens: cr.Metrics.EvalCount,
		}
		return nil
	}); err != nil {
		return "", usage, err
	}

	return full, usage, nil
}

func (o *Ollama) Stream(ctx context.Context, msgs []types.Message, cb func(types.Chunk)) (types.Usage, error) {
	om := make([]api.Message, len(msgs))
	for i, m := range msgs {
		om[i] = api.Message{Role: string(m.Role), Content: m.Content}
	}
	stream := true
	req := &api.ChatRequest{Model: o.model, Messages: om, Stream: &stream}

	var usage types.Usage
	if err := o.client.Chat(ctx, req, func(cr api.ChatResponse) error {
		token := cr.Message.Content
		if token == "" {
			return nil
		}
		cb(types.Chunk{Content: token, Delta: 1}) // Ollama 无法精确分词，这里假设 1
		usage.CompletionTokens += 1
		usage.PromptTokens = cr.Metrics.PromptEvalCount
		return nil
	}); err != nil {
		return usage, err
	}
	return usage, nil
}

// 在 init 中注册到全局表，实现“热插拔”
func init() {
	provider.Register("ollama", New("llama3"))
}
