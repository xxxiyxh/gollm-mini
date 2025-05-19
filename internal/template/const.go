package template

const (
	DefaultSystem = "You are a helpful assistant."
)

type Parts struct {
	Context    string `json:"context,omitempty"`     // 场景背景
	Directives string `json:"directives,omitempty"`  // 额外规则
	OutputHint string `json:"output_hint,omitempty"` // 输出格式或语言
	MaxLen     int    `json:"max_len,omitempty"`     // 预估最大 token
}
