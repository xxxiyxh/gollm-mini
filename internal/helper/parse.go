package helper

import (
	"regexp"
	"strconv"
)

func ParseFloat(s string) float64 {
	re := regexp.MustCompile(`\d+(\.\d+)?`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}
	f, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return f
}
