package helper

// 单位：USD / 1K tokens
var priceTable = map[string]struct {
	Prompt, Completion float64
}{
	"openai:gpt-4o-mini":   {0.005, 0.015},
	"openai:gpt-3.5-turbo": {0.0005, 0.0015},
	// 本地 Ollama 视为 0
}

func CalcCost(provider, model string, promptTok, compTok int) float64 {
	key := provider + ":" + model
	p, ok := priceTable[key]
	if !ok {
		return 0
	}
	return (float64(promptTok)*p.Prompt + float64(compTok)*p.Completion) / 1000
}
