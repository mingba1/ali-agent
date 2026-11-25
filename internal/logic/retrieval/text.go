package retrieval

import (
	"context"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/model"
	"ali-agent/internal/utility/keyword"
)

// TextSearch 文本数据库检索：关键词 LIKE 匹配（title/tags/heading/content），加权评分。
func TextSearch(ctx context.Context, query string, topK int) ([]model.Hit, error) {
	kws := normalize(keyword.Extract(query))
	if len(kws) == 0 {
		return []model.Hit{}, nil
	}
	titleW := int(g.Cfg().MustGet(ctx, "retrieval.text.titleWeight", 3).Int())
	tagW := int(g.Cfg().MustGet(ctx, "retrieval.text.tagWeight", 2).Int())
	contentW := int(g.Cfg().MustGet(ctx, "retrieval.text.contentWeight", 1).Int())

	conds := make([]string, 0, len(kws))
	args := make([]interface{}, 0, len(kws)*4)
	for _, kw := range kws {
		like := "%" + kw + "%"
		conds = append(conds, "(c.content LIKE ? OR c.heading LIKE ? OR d.title LIKE ? OR d.tags LIKE ?)")
		args = append(args, like, like, like, like)
	}
	type row struct {
		ChunkId, DocId                int64
		Heading, Content, Title, Tags string
		StartOffset, EndOffset        int
	}
	sql := `SELECT c.id AS chunk_id, c.doc_id, c.heading, c.content, c.start_offset, c.end_offset,
	               d.title, d.tags
	        FROM doc_chunk c INNER JOIN doc d ON d.id = c.doc_id
	        WHERE ` + strings.Join(conds, " OR ") + ` LIMIT 200`
	var rows []row
	result, err := g.DB().GetAll(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if err = result.Structs(&rows); err != nil {
		return nil, err
	}

	hits := make([]model.Hit, 0, len(rows))
	for _, r := range rows {
		score := 0.0
		for _, kw := range kws {
			lk := strings.ToLower(kw)
			score += float64(strings.Count(strings.ToLower(r.Title), lk)*titleW +
				strings.Count(strings.ToLower(r.Tags), lk)*tagW +
				strings.Count(strings.ToLower(r.Heading), lk)*tagW +
				strings.Count(strings.ToLower(r.Content), lk)*contentW)
		}
		if score <= 0 {
			continue
		}
		hits = append(hits, model.Hit{
			Source: "text", DocId: r.DocId, ChunkId: r.ChunkId,
			Title: r.Title, Heading: r.Heading, Tags: r.Tags,
			Content:     snippet(r.Content, kws, 160),
			StartOffset: r.StartOffset, EndOffset: r.EndOffset,
			Score: score, Ref: "doc:" + itoa(r.DocId) + "#chunk:" + itoa(r.ChunkId),
		})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ChunkId < hits[j].ChunkId
		}
		return hits[i].Score > hits[j].Score
	})
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

func normalize(in []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(in))
	for _, kw := range in {
		kw = strings.TrimSpace(strings.ToLower(kw))
		if kw == "" {
			continue
		}
		if _, ok := seen[kw]; ok {
			continue
		}
		seen[kw] = struct{}{}
		out = append(out, kw)
	}
	return out
}
