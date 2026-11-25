package retrieval

import (
	"context"
	"sort"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/model"
	"ali-agent/internal/utility/embedding"
	"ali-agent/internal/utility/keyword"
)

// VectorSearch 向量数据库检索：查询 embedding → 全量余弦相似度 → 过滤 → top-k。
// 生产环境应替换为 ANN 向量库（pgvector/Milvus/Qdrant）。
func VectorSearch(ctx context.Context, query string, topK int) ([]model.Hit, error) {
	emb := embedding.New(ctx)
	vecs, err := emb.Embed(ctx, []string{query})
	if err != nil || len(vecs) == 0 {
		return nil, err
	}
	queryVec := vecs[0]

	minScore := g.Cfg().MustGet(ctx, "retrieval.vector.minScore", 0.08).Float64()
	type row struct {
		ChunkId, DocId                int64
		Heading, Content, Title, Tags string
		StartOffset, EndOffset        int
		Embedding                     string
	}
	var rows []row
	result, err := g.DB().GetAll(ctx,
		`SELECT c.id AS chunk_id, c.doc_id, c.heading, c.content, c.start_offset, c.end_offset,
		        c.embedding, d.title, d.tags
		 FROM doc_chunk c INNER JOIN doc d ON d.id = c.doc_id`)
	if err != nil {
		return nil, err
	}
	if err = result.Structs(&rows); err != nil {
		return nil, err
	}

	hits := make([]model.Hit, 0, len(rows))
	for _, r := range rows {
		vec := embedding.Decode(r.Embedding)
		if len(vec) == 0 {
			continue
		}
		sim := embedding.Cosine(queryVec, vec)
		if sim < minScore {
			continue
		}
		hits = append(hits, model.Hit{
			Source: "vector", DocId: r.DocId, ChunkId: r.ChunkId,
			Title: r.Title, Heading: r.Heading, Tags: r.Tags,
			Content:     snippet(r.Content, keyword.Extract(query), 160),
			StartOffset: r.StartOffset, EndOffset: r.EndOffset,
			Score: sim, Ref: "doc:" + itoa(r.DocId) + "#chunk:" + itoa(r.ChunkId),
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}
