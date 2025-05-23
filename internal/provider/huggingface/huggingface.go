package huggingface

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gollm-mini/internal/provider"
	"gollm-mini/internal/types"
)

type HF struct {
	client  *http.Client
	apiKey  string
	modelID string
	baseURL string
}

// ---------------------------------------------------------------------
// 构造 & 配置
// ---------------------------------------------------------------------

func New(model string) *HF {
	key := os.Getenv("HF_API_KEY")   // 远端调用时需要
	base := os.Getenv("HF_BASE_URL") // 允许覆盖
	if base == "" {
		base = "https://api-inference.huggingface.co/models"
	}
	return &HF{
		client:  &http.Client{Timeout: 60 * time.Second},
		apiKey:  key,
		modelID: model,
		baseURL: strings.TrimRight(base, "/"),
	}
}

func (h *HF) SetModel(m string) { h.modelID = m }

// ---------------------------------------------------------------------
// 核心：Generate
// ---------------------------------------------------------------------

func (h *HF) Generate(ctx context.Context, msgs []types.Message) (string, types.Usage, error) {
	// -------------------- 1) 参数检查 --------------------
	isRemote := strings.Contains(h.baseURL, "api-inference.huggingface.co")
	if isRemote && h.apiKey == "" {
		return "", types.Usage{}, errors.New("HF_API_KEY not set (remote HF API)")
	}

	// -------------------- 2) 拼 prompt --------------------
	prompt := buildPrompt(msgs)

	// -------------------- 3) 计算 URL --------------------
	url := h.baseURL
	if isRemote {
		// 远端：BASE/models/<model>
		url = fmt.Sprintf("%s/%s", h.baseURL, h.modelID)
	} else {
		// 本地：确保以 /generate 结尾
		if !strings.HasSuffix(h.baseURL, "/generate") {
			url = h.baseURL + "/generate"
		}
	}

	// -------------------- 4) 构造请求体 --------------------
	var payload any
	if isRemote {
		payload = map[string]string{"inputs": prompt}
	} else {
		payload = map[string]string{
			"input": prompt,
			"model": h.modelID, // 便于 FastAPI 端动态加载
		}
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if isRemote {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	// -------------------- 5) 发送 --------------------

	resp, err := h.client.Do(req)
	if err != nil {
		return "", types.Usage{}, err
	}
	defer resp.Body.Close()

	// 常见重试场景：503 正在加载权重
	if resp.StatusCode == 503 {
		return "", types.Usage{}, errors.New("model loading on HF, retry later")
	}
	if resp.StatusCode != 200 {
		return "", types.Usage{}, fmt.Errorf("HF API %s", resp.Status)
	}

	// -------------------- 6) 解析响应 --------------------
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", types.Usage{}, err
	}

	// 6-a 远端格式：[{ "generated_text": "..." }]
	var arr []struct {
		GeneratedText string `json:"generated_text"`
	}
	if json.Unmarshal(respBytes, &arr) == nil && len(arr) > 0 {
		txt := strings.TrimSpace(arr[0].GeneratedText)
		usage := types.Usage{
			PromptTokens:     approxTokens(prompt),
			CompletionTokens: approxTokens(txt),
		}
		return txt, usage, nil
	}

	// 6-b 本地格式：{ "text": "..." }
	var obj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBytes, &obj); err != nil {
		return "", types.Usage{}, fmt.Errorf("decode HF response: %w", err)
	}
	txt := postProcess(obj.Text)
	usage := types.Usage{
		PromptTokens:     approxTokens(prompt),
		CompletionTokens: approxTokens(txt),
	}
	return txt, usage, nil
}

// ---------------------------------------------------------------------
// Stream：沿用你原来的“空格伪流”实现
// ---------------------------------------------------------------------

func (h *HF) Stream(ctx context.Context, msgs []types.Message, cb func(types.Chunk)) (types.Usage, error) {
	txt, usage, err := h.Generate(ctx, msgs)
	if err != nil {
		return usage, err
	}
	for _, tok := range strings.Split(txt, " ") {
		select {
		case <-ctx.Done():
			return usage, ctx.Err()
		default:
			cb(types.Chunk{Content: tok + " ", Delta: 1})
			time.Sleep(60 * time.Millisecond)
		}
	}
	return usage, nil
}

// ---------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------

func buildPrompt(msgs []types.Message) string {
	var sys, user string
	for _, m := range msgs {
		switch m.Role {
		case types.RoleSystem:
			sys = m.Content
		case types.RoleUser:
			user = m.Content
		}
	}
	if sys == "" {
		sys = "You are a helpful assistant."
	}
	return fmt.Sprintf(
		"<s>"+
			"<|system|>\n%s</s>\n"+
			"<|user|>\n%s</s>\n"+
			"<|assistant|>\n",
		sys, user,
	)

}

// 本地返回值里可能带多余 tag，简单裁剪一下
func postProcess(txt string) string {
	// ---------- 1) 保留 Assistant 之后的内容 ----------
	if i := strings.Index(txt, "<|assistant|>"); i != -1 {
		txt = txt[i+len("<|assistant|>"):]
	}

	// ---------- 2) 去掉前缀 <s>（TinyLlama 有时会给两个） ----------
	txt = strings.TrimSpace(txt)
	for strings.HasPrefix(txt, "<s>") {
		txt = strings.TrimPrefix(txt, "<s>")
		txt = strings.TrimSpace(txt)
	}

	// ---------- 3) 删掉结尾 "</s>" 及其后噪声 ----------
	if j := strings.Index(txt, "</s>"); j != -1 {
		txt = txt[:j]
	}

	// ---------- 收尾 ----------
	return strings.TrimSpace(txt)
}

// 粗略 token 估算：按空格数
func approxTokens(s string) int { return len(strings.Split(s, " ")) }

// ---------------------------------------------------------------------
// 注册到 provider 工厂
// ---------------------------------------------------------------------

func init() {
	provider.Register("hf", New("TinyLlama/TinyLlama-1.1B-Chat-v1.0"))
}
