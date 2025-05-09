package openai

import (
	"context"
	"io"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"gollm-mini/internal/provider"
	"gollm-mini/internal/types"
)

type OpenAI struct {
	client *openai.Client
	model  string
}

func (o *OpenAI) SetModel(m string) { o.model = m }

func New(model string) *OpenAI {
	return &OpenAI{
		client: openai.NewClient(os.Getenv("OPENAI_API_KEY")),
		model:  model,
	}
}

// ----------- 非流式 --------------------------------------------------------

func (o *OpenAI) Generate(ctx context.Context, msgs []types.Message) (string, types.Usage, error) {
	req := o.buildRequest(msgs, false)

	resp, err := o.client.CreateChatCompletion(ctx, *req)
	if err != nil {
		return "", types.Usage{}, err
	}

	u := types.Usage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
	}
	return resp.Choices[0].Message.Content, u, nil
}

// ----------- 流式 ----------------------------------------------------------

func (o *OpenAI) Stream(
	ctx context.Context,
	msgs []types.Message,
	cb func(types.Chunk),
) (types.Usage, error) {

	req := o.buildRequest(msgs, true)

	stream, err := o.client.CreateChatCompletionStream(ctx, *req)
	if err != nil {
		return types.Usage{}, err
	}
	defer stream.Close()

	var usage types.Usage

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF { // 流结束
				break
			}
			return usage, err
		}

		if len(resp.Choices) == 0 {
			continue
		}

		delta := resp.Choices[0].Delta.Content
		if delta == "" {
			continue
		}

		cb(types.Chunk{Content: delta, Delta: 1}) // 每块按 1 token 计
		usage.CompletionTokens++
	}

	// PromptTokens 暂无法从 SDK 拿到，可在此处自行调用 tiktoken 计算
	usage.PromptTokens = 0
	return usage, nil
}

// ----------- 工具 & 注册 ----------------------------------------------------

func (o *OpenAI) buildRequest(msgs []types.Message, stream bool) *openai.ChatCompletionRequest {
	cm := make([]openai.ChatCompletionMessage, len(msgs))
	for i, m := range msgs {
		cm[i] = openai.ChatCompletionMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}
	return &openai.ChatCompletionRequest{
		Model:    o.model,
		Messages: cm,
		Stream:   stream,
	}
}

func init() {
	provider.Register("openai", New("gpt-3.5-turbo"))
}
