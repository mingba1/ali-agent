package retrieval

import (
	"sort"
	"strings"

	"ali-agent/internal/model"
	"ali-agent/internal/utility/keyword"
)

// Rerank 对多路召回结果做重排：
//  1. 按来源归一化分数
//  2. 用关键词覆盖率加权（简单的 cross-encoder 替代，零依赖）
//  3. 同文档去重，保留最高分
//
// 生产环境应替换为 cross-encoder 重排模型（如 bge-reranker）。
func Rerank(query string, hits []model.Hit, topK int) []model.Hit {
	if len(hits) == 0 {
		return hits
	}
	kws := keyword.Extract(query)
	if len(kws) == 0 {
		kws = strings.Fields(strings.ToLower(query))
	}
	// 按来源归一化原始分：每个来源内 max-min 归一化
	bySource := make(map[string][]model.Hit)
	for _, h := range hits {
		bySource[h.Source] = append(bySource[h.Source], h)
	}
	normScore := make(map[string]float64)
	for _, list := range bySource {
		var mx, mn float64
		for i, h := range list {
			if i == 0 || h.Score > mx {
				mx = h.Score
			}
			if i == 0 || h.Score < mn {
				mn = h.Score
			}
		}
		span := mx - mn
		if span == 0 {
			for _, h := range list {
				normScore[h.Ref] = 1.0
			}
			continue
		}
		for _, h := range list {
			normScore[h.Ref] = (h.Score - mn) / span
		}
	}

	// 关键词覆盖率
	cov := func(text string) float64 {
		if len(kws) == 0 {
			return 0
		}
		t := strings.ToLower(text)
		hit := 0
		for _, kw := range kws {
			if strings.Contains(t, kw) {
				hit++
			}
		}
		return float64(hit) / float64(len(kws))
	}

	seen := make(map[string]struct{})
	out := make([]model.Hit, 0, len(hits))
	for _, h := range hits {
		key := h.Ref
		if key == "" {
			key = h.Source + ":" + itoa(h.DocId) + ":" + itoa(h.ChunkId)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		h.RerankScore = normScore[h.Ref]*0.6 + cov(h.Title+" "+h.Heading+" "+h.Content+" "+h.Row)*0.4
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RerankScore > out[j].RerankScore })
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}
