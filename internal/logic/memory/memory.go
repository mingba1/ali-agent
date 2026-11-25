package memory

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/guid"

	"ali-agent/internal/model"
)

// System 对应架构图中的 memory system：Orchestrator 可读写的会话记忆与草稿板。
type System struct {
	sessionId string
	history   []model.ChatMessage
}

// Load 加载会话历史（最近 limit 条），不存在则创建。
func Load(ctx context.Context, sessionId, userId, query string, limit int) *System {
	if sessionId == "" {
		sessionId = "s_" + guid.S()
	}
	count, _ := g.Model("chat_session").Ctx(ctx).Where("id", sessionId).Count()
	if count == 0 {
		_, _ = g.Model("chat_session").Ctx(ctx).Data(g.Map{
			"id": sessionId, "user_id": userId, "title": truncate(query, 60),
		}).Insert()
	}
	list := make([]model.ChatMessage, 0)
	_ = g.Model("chat_message").Ctx(ctx).Where("session_id", sessionId).
		OrderDesc("id").Limit(limit).Scan(&list)
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return &System{sessionId: sessionId, history: list}
}

func (m *System) SessionId() string { return m.sessionId }

// HistoryText 把历史对话格式化成文本供 LLM 参考。
func (m *System) HistoryText() string {
	if len(m.history) == 0 {
		return "（无）"
	}
	var b strings.Builder
	for _, msg := range m.history {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}
	return b.String()
}

// Save 持久化本轮问答。
func (m *System) Save(ctx context.Context, query, answer string) {
	_, _ = g.Model("chat_message").Ctx(ctx).Data(g.List{
		g.Map{"session_id": m.sessionId, "role": "user", "content": query},
		g.Map{"session_id": m.sessionId, "role": "assistant", "content": answer},
	}).Insert()
	_, _ = g.Model("chat_session").Ctx(ctx).Where("id", m.sessionId).
		Data(g.Map{"updated_at": time.Now()}).Update()
}

// Scratchpad 把多轮观察整理成草稿板文本，供下一轮思考参考。
func Scratchpad(steps []model.LoopStep) string {
	var b strings.Builder
	for _, step := range steps {
		b.WriteString("第 ")
		b.WriteString(itoa(int64(step.Round)))
		b.WriteString("轮：")
		b.WriteString(step.Thought)
		b.WriteString("\n")
		for _, obs := range step.Observations {
			b.WriteString("  [")
			b.WriteString(obs.Agent)
			b.WriteString("] ")
			b.WriteString(obs.Observation)
			b.WriteString("\n")
			for _, h := range obs.Hits {
				b.WriteString("    - ")
				b.WriteString(h.Ref)
				b.WriteString(": ")
				b.WriteString(truncate(h.Title+" "+h.Heading+" "+h.Content+" "+h.Row, 100))
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func truncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "..."
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }
