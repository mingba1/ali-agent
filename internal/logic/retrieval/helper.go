package retrieval

import (
	"strconv"
	"strings"
)

// itoa int64 -> string。
func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// snippet 从内容中截取包含关键词的片段。
func snippet(content string, keywords []string, limit int) string {
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	index := -1
	if len(keywords) > 0 {
		lower := strings.ToLower(content)
		for _, kw := range keywords {
			if i := strings.Index(lower, strings.ToLower(kw)); i >= 0 && (index < 0 || i < index) {
				index = i
			}
		}
	}
	start := 0
	if index >= 0 {
		start = len([]rune(content[:index])) - limit/3
		if start < 0 {
			start = 0
		}
	}
	end := start + limit
	if end > len(runes) {
		end = len(runes)
	}
	text := string(runes[start:end])
	if start > 0 {
		text = "..." + text
	}
	if end < len(runes) {
		text += "..."
	}
	return text
}
