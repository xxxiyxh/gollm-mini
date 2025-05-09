package types

type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

func (u Usage) Total() int {
	return u.PromptTokens + u.CompletionTokens
}
