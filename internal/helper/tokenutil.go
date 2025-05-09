package helper

import (
	"gollm-mini/internal/types"
	"unicode/utf8"
)

// RoughTokenCount ＝词数近似；80% 情况够用，后续可换 tiktoken
func RoughTokenCount(s string) int {
	return utf8.RuneCountInString(s) / 4
}

// TruncateMessages 保留 history 尾部，直至 token 总量 ≤ limit
func TruncateMessages(msgs []types.Message, limit int) []types.Message {
	var total int
	// 从后往前累加
	for i := len(msgs) - 1; i >= 0; i-- {
		total += RoughTokenCount(msgs[i].Content)
		if total > limit {
			return msgs[i+1:]
		}
	}
	return msgs
}
