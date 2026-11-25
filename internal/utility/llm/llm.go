package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

type Client struct {
	apiKey   string
	baseURL  string
	model    string
	temp     float64
	timeout  time.Duration
	mockMode bool
}

func New(ctx context.Context) *Client {
	apiKey := strings.TrimSpace(g.Cfg().MustGet(ctx, "llm.apiKey", "").String())
	timeout := g.Cfg().MustGet(ctx, "llm.timeoutSeconds", 60).Int()
	if timeout <= 0 {
		timeout = 60
	}
	temp := g.Cfg().MustGet(ctx, "llm.temperature", 0.3).Float64()
	return &Client{
		apiKey:   apiKey,
		baseURL:  strings.TrimRight(g.Cfg().MustGet(ctx, "llm.baseUrl", "https://api.openai.com/v1").String(), "/"),
		model:    g.Cfg().MustGet(ctx, "llm.model", "gpt-4o-mini").String(),
		temp:     temp,
		timeout:  time.Duration(timeout) * time.Second,
		mockMode: apiKey == "",
	}
}

func (c *Client) Enabled() bool { return !c.mockMode }

// Chat 发送普通对话请求，返回文本。
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if c.mockMode {
		return mockReply(system, user), nil
	}
	body, err := json.Marshal(map[string]interface{}{
		"model": c.model, "temperature": c.temp,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := (&http.Client{Timeout: c.timeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse llm response failed: %s", string(raw))
	}
	if resp.StatusCode != http.StatusOK {
		if result.Error != nil {
			return "", fmt.Errorf("llm call failed(%d): %s", resp.StatusCode, result.Error.Message)
		}
		return "", fmt.Errorf("llm call failed(%d)", resp.StatusCode)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

// ChatJSON 发送对话并解析返回 JSON 到 out。
func (c *Client) ChatJSON(ctx context.Context, system, user string, out interface{}) error {
	raw, err := c.Chat(ctx, system, user)
	if err != nil {
		return err
	}
	return parseJSON(raw, out)
}

// parseJSON 从模型回复中提取 JSON（兼容 ```json 包裹或前后夹杂文本）。
func parseJSON(raw string, out interface{}) error {
	raw = strings.TrimSpace(raw)
	// 去除 ```json ... ``` 包裹
	if idx := strings.Index(raw, "```"); idx >= 0 {
		start := strings.Index(raw[idx:], "\n")
		if start >= 0 {
			raw = raw[idx+start+1:]
		}
		if end := strings.Index(raw, "```"); end >= 0 {
			raw = raw[:end]
		}
	}
	raw = strings.TrimSpace(raw)
	// 截取第一个 { 到最后一个 }
	first := strings.Index(raw, "{")
	last := strings.LastIndex(raw, "}")
	if first >= 0 && last > first {
		raw = raw[first : last+1]
	}
	return json.Unmarshal([]byte(raw), out)
}

// mockReply 模拟模型回复，保证离线可跑。
func mockReply(system, user string) string {
	switch {
	case strings.Contains(system, "Orchestrator"):
		// 已有观察中包含重排结果时，直接给出答复；否则 fan-out 全部子 Agent。
		if strings.Contains(user, "重排") || strings.Contains(user, "rerank") {
			return `{"action":"answer","answer":"基于多路检索证据整理的回答。","reason":"已有足够证据，生成最终答复"}`
		}
		return `{"action":"fanout","subAgents":["vector","text","fs","db"],"reason":"多路并行检索以覆盖知识库、文件系统与数据库"}`
	case strings.Contains(system, "Vector") || strings.Contains(system, "vector"):
		return `{"tool":"vector_search","query":"` + clean(user) + `","topK":10}`
	case strings.Contains(system, "Text") || strings.Contains(system, "text"):
		return `{"tool":"text_search","query":"` + clean(user) + `","topK":10}`
	case strings.Contains(system, "File") || strings.Contains(system, "grep"):
		return `{"tool":"grep","query":"` + clean(user) + `","topK":20}`
	case strings.Contains(system, "DB") || strings.Contains(system, "SQL") || strings.Contains(system, "数据库"):
		// Mock 模式下用一个只读查询演示 OLTP/OLAP 后端通路。
		return `{"tool":"sql","query":"SELECT id, title, tags, author FROM doc ORDER BY id DESC LIMIT 30","topK":30}`
	case strings.Contains(system, "Writer") || strings.Contains(system, "writer"):
		return fmt.Sprintf("（Mock Writer）基于检索证据整理的回答：\n%s", user)
	default:
		return "(mock) " + user
	}
}

func clean(s string) string {
	r := strings.NewReplacer("\"", "'", "\n", " ", "\r", "")
	return r.Replace(strings.TrimSpace(s))
}
