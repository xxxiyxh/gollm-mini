package huggingface

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gollm-mini/internal/provider"
	"gollm-mini/internal/types"
)

type HuggingFace struct {
	client  *http.Client
	apiKey  string
	modelID string
}

func New(modelID string) *HuggingFace {
	return &HuggingFace{
		client:  &http.Client{Timeout: 60 * time.Second},
		apiKey:  os.Getenv("HF_API_KEY"),
		modelID: modelID,
	}
}

func (h *HuggingFace) SetModel(m string) { h.modelID = m }

func (h *HuggingFace) Generate(ctx context.Context, msgs []types.Message) (string, types.Usage, error) {
	// 拼接历史消息
	var prompt string
	for _, m := range msgs {
		prompt += fmt.Sprintf("[%s] %s\n", m.Role, m.Content)
	}

	reqBody, _ := json.Marshal(map[string]string{"inputs": prompt})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api-inference.huggingface.co/models/"+h.modelID, bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+h.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", types.Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", types.Usage{}, fmt.Errorf("HF error: %s", body)
	}

	var result []map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", types.Usage{}, err
	}

	if len(result) == 0 {
		return "", types.Usage{}, fmt.Errorf("HF: no output")
	}
	return result[0]["generated_text"], types.Usage{}, nil
}

// Stream hf大部分不支持流式输出，为实现接口定义
func (h *HuggingFace) Stream(ctx context.Context, msgs []types.Message, cb func(types.Chunk)) (types.Usage, error) {
	// 实际调用一次 Generate
	text, usage, err := h.Generate(ctx, msgs)
	if err != nil {
		return usage, err
	}

	// 模拟逐 token 回调
	tokens := strings.Split(text, " ") // 简化：按空格模拟 token
	for _, token := range tokens {
		select {
		case <-ctx.Done():
			return usage, ctx.Err()
		default:
			cb(types.Chunk{Content: token + " ", Delta: 1})
			time.Sleep(60 * time.Millisecond) // 控制节奏
		}
	}
	return usage, nil
}

func init() {
	provider.Register("huggingface", New("bigscience/bloom"))
}
