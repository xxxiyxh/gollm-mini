package helper

import (
	"strconv"
	"strings"
)

// ParseFloat 解析模型返回的打分文本，容错处理，如 "Score: 9"
func ParseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ":") // 去掉尾部冒号
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
