package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/utility/keyword"
)

const defaultDim = 256

type Embedder struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	timeout time.Duration
}

func New(ctx context.Context) *Embedder {
	dim := g.Cfg().MustGet(ctx, "embedding.dim", defaultDim).Int()
	if dim <= 0 {
		dim = defaultDim
	}
	timeout := g.Cfg().MustGet(ctx, "embedding.timeoutSeconds", 30).Int()
	if timeout <= 0 {
		timeout = 30
	}
	return &Embedder{
		apiKey:  firstNonEmpty(os.Getenv("EMBEDDING_API_KEY"), g.Cfg().MustGet(ctx, "embedding.apiKey", "").String()),
		baseURL: strings.TrimRight(firstNonEmpty(os.Getenv("EMBEDDING_BASE_URL"), g.Cfg().MustGet(ctx, "embedding.baseUrl", "https://api.openai.com/v1").String()), "/"),
		model:   firstNonEmpty(os.Getenv("EMBEDDING_MODEL"), g.Cfg().MustGet(ctx, "embedding.model", "text-embedding-3-small").String()),
		dim:     dim,
		timeout: time.Duration(timeout) * time.Second,
	}
}

func (e *Embedder) Mode() string {
	if e.apiKey != "" {
		return "api"
	}
	return "local-hash"
}

func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if e.apiKey != "" {
		return e.embedAPI(ctx, texts)
	}
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = localEmbed(text, e.dim)
	}
	return out, nil
}

func (e *Embedder) embedAPI(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]interface{}{"model": e.model, "input": texts}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	resp, err := (&http.Client{Timeout: e.timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse embedding response failed: %s", truncate(string(raw), 200))
	}
	if resp.StatusCode != http.StatusOK {
		if result.Error != nil {
			return nil, fmt.Errorf("embedding call failed(%d): %s", resp.StatusCode, result.Error.Message)
		}
		return nil, fmt.Errorf("embedding call failed(%d)", resp.StatusCode)
	}
	sort.Slice(result.Data, func(i, j int) bool { return result.Data[i].Index < result.Data[j].Index })
	out := make([][]float32, len(result.Data))
	for i, item := range result.Data {
		out[i] = item.Embedding
	}
	return out, nil
}

func localEmbed(text string, dim int) []float32 {
	vec := make([]float32, dim)
	tokens := keyword.Extract(text)
	runes := []rune(text)
	if len(runes) > 800 {
		runes = runes[:800]
	}
	for i := 0; i+2 < len(runes); i++ {
		tokens = append(tokens, string(runes[i:i+3]))
	}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(token))
		v := h.Sum32()
		idx := int(v % uint32(dim))
		if v&0x80000000 != 0 {
			vec[idx] -= 1
		} else {
			vec[idx] += 1
		}
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v * v)
	}
	if norm == 0 {
		return vec
	}
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] = float32(float64(vec[i]) / norm)
	}
	return vec
}

func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func Encode(vec []float32) string {
	raw, _ := json.Marshal(vec)
	return string(raw)
}

func Decode(text string) []float32 {
	if text == "" {
		return nil
	}
	var vec []float32
	_ = json.Unmarshal([]byte(text), &vec)
	return vec
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "..."
}
