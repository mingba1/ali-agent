package keyword

import (
	"regexp"
	"strings"
)

var (
	latinRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_\-]{1,}`)
	cjkRe   = regexp.MustCompile(`[\p{Han}]+`)
)

var stopBigrams = map[string]struct{}{
	"怎么": {}, "如何": {}, "什么": {}, "可以": {}, "一下": {}, "我们": {},
	"请问": {}, "是否": {}, "哪些": {}, "这个": {}, "那个": {}, "现在": {},
	"需要": {}, "应该": {}, "帮我": {}, "告诉": {}, "介绍": {}, "有没": {},
	"没有": {}, "以及": {}, "或者": {}, "还是": {}, "相关": {}, "关于": {},
	"我要": {}, "想要": {}, "能否": {}, "多少": {}, "为什": {}, "么样": {},
	"最新": {}, "新闻": {}, "网页": {}, "官网": {}, "链接": {}, "看下": {},
}

var englishStop = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "what": {}, "how": {}, "why": {},
	"this": {}, "that": {}, "with": {}, "from": {}, "are": {}, "you": {},
	"can": {}, "does": {}, "did": {}, "was": {}, "were": {}, "will": {},
}

// Extract 从自然语言问题中抽取检索关键词。
func Extract(query string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 10)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, w := range latinRe.FindAllString(strings.ToLower(query), -1) {
		if _, ok := englishStop[w]; ok {
			continue
		}
		add(w)
	}
	for _, run := range cjkRe.FindAllString(query, -1) {
		rs := []rune(run)
		if len(rs) <= 2 {
			if _, ok := stopBigrams[string(rs)]; !ok {
				add(string(rs))
			}
			continue
		}
		for i := 0; i+1 < len(rs); i++ {
			bigram := string(rs[i : i+2])
			if _, ok := stopBigrams[bigram]; ok {
				continue
			}
			add(bigram)
		}
	}
	if len(out) == 0 {
		add(strings.TrimSpace(query))
	}
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
