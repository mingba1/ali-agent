package doc

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/model"
	"ali-agent/internal/service"
	"ali-agent/internal/utility/embedding"
)

type sDoc struct{}

func init() { service.RegisterDoc(&sDoc{}) }

const (
	maxChunkRunes = 400
	minChunkRunes = 12
	overlapRunes  = 50
)

type heading struct {
	level int
	text  string
}
type chunkDraft struct {
	heading string
	content string
	start   int
	end     int
}

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

func splitChunks(content string) []chunkDraft {
	runes := []rune(content)
	drafts := make([]chunkDraft, 0)
	stack := make([]heading, 0, 4)
	curStart, curEnd, curHeading := -1, -1, ""
	flush := func() {
		if curStart >= 0 {
			start, end := trimRange(runes, curStart, curEnd)
			if end > start {
				drafts = append(drafts, chunkDraft{
					heading: curHeading, content: string(runes[start:end]),
					start: start, end: end,
				})
			}
		}
		curStart, curEnd, curHeading = -1, -1, ""
	}
	lineStart := 0
	for lineStart <= len(runes) {
		lineEnd := lineStart
		for lineEnd < len(runes) && runes[lineEnd] != '\n' {
			lineEnd++
		}
		line := strings.TrimSpace(string(runes[lineStart:lineEnd]))
		switch {
		case line == "":
			flush()
		case headingRe.MatchString(line):
			flush()
			m := headingRe.FindStringSubmatch(line)
			level := len(m[1])
			text := strings.TrimSpace(m[2])
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, heading{level: level, text: text})
		default:
			if curStart < 0 {
				curStart = lineStart
				curHeading = headingPath(stack)
			}
			curEnd = lineEnd
		}
		if lineEnd >= len(runes) {
			break
		}
		lineStart = lineEnd + 1
	}
	flush()
	out := make([]chunkDraft, 0, len(drafts))
	for _, d := range drafts {
		out = append(out, splitLong(d)...)
	}
	return out
}

func splitLong(d chunkDraft) []chunkDraft {
	runes := []rune(d.content)
	if len(runes) <= maxChunkRunes {
		return []chunkDraft{d}
	}
	out := make([]chunkDraft, 0, len(runes)/maxChunkRunes+1)
	for i := 0; i < len(runes); {
		end := i + maxChunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		if end < len(runes) {
			for j := end; j > i+maxChunkRunes/2; j-- {
				if isSentenceEnd(runes[j-1]) {
					end = j
					break
				}
			}
		}
		textStart, textEnd := trimRange(runes, i, end)
		if textEnd-textStart >= minChunkRunes {
			out = append(out, chunkDraft{
				heading: d.heading, content: string(runes[textStart:textEnd]),
				start: d.start + textStart, end: d.start + textEnd,
			})
		}
		if end >= len(runes) {
			break
		}
		next := end - overlapRunes
		if next <= i {
			next = end
		}
		i = next
	}
	return out
}

func trimRange(runes []rune, start, end int) (int, int) {
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	for start < end && unicode.IsSpace(runes[start]) {
		start++
	}
	for end > start && unicode.IsSpace(runes[end-1]) {
		end--
	}
	return start, end
}

func headingPath(stack []heading) string {
	if len(stack) == 0 {
		return ""
	}
	parts := make([]string, 0, len(stack))
	for _, h := range stack {
		parts = append(parts, h.text)
	}
	return strings.Join(parts, " / ")
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '！', '？', '；', '!', '?', ';', '\n':
		return true
	}
	return false
}

// Reindex 重建全部分块与向量索引。
func (s *sDoc) Reindex(ctx context.Context) (int, int, error) {
	docs := make([]model.Doc, 0)
	if err := g.Model("doc").Ctx(ctx).OrderAsc("id").Scan(&docs); err != nil {
		return 0, 0, err
	}
	emb := embedding.New(ctx)
	docCount, chunkCount := 0, 0
	for _, item := range docs {
		if _, err := g.Model("doc_chunk").Ctx(ctx).Where("doc_id", item.Id).Delete(); err != nil {
			return 0, 0, err
		}
		drafts := splitChunks(item.Content)
		if len(drafts) == 0 {
			continue
		}
		texts := make([]string, len(drafts))
		for i, d := range drafts {
			texts[i] = item.Title
			if d.heading != "" {
				texts[i] += "\n" + d.heading
			}
			texts[i] += "\n" + d.content
		}
		vectors, err := emb.Embed(ctx, texts)
		if err != nil {
			return 0, 0, err
		}
		list := make(g.List, 0, len(drafts))
		for i, d := range drafts {
			embText := ""
			if i < len(vectors) {
				embText = embedding.Encode(vectors[i])
			}
			list = append(list, g.Map{
				"doc_id": item.Id, "chunk_index": i,
				"heading": d.heading, "content": d.content,
				"start_offset": d.start, "end_offset": d.end,
				"embedding": embText,
			})
		}
		if _, err = g.Model("doc_chunk").Ctx(ctx).Data(list).Insert(); err != nil {
			return 0, 0, err
		}
		docCount++
		chunkCount += len(drafts)
	}
	return docCount, chunkCount, nil
}
