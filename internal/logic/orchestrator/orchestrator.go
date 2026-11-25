package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/logic/memory"
	"ali-agent/internal/logic/retrieval"
	"ali-agent/internal/logic/subagent"
	"ali-agent/internal/model"
	"ali-agent/internal/service"
	"ali-agent/internal/utility/llm"
)

type sChat struct{}

func init() { service.RegisterChat(&sChat{}) }

// Chat 执行 Orchestrator Agent Loop：
// 思考 → 决定 fan-out 到哪些子 Agent → 并行执行 → 观察 → 再思考 → 回答。
func (s *sChat) Chat(ctx context.Context, in model.ChatInput) (*model.ChatResult, error) {
	start := time.Now()
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, gerror.New("问题不能为空")
	}
	maxRounds := g.Cfg().MustGet(ctx, "agent.maxRounds", 4).Int()
	if maxRounds <= 0 {
		maxRounds = 4
	}
	memoryLimit := g.Cfg().MustGet(ctx, "agent.memoryLimit", 10).Int()
	rerankTopK := g.Cfg().MustGet(ctx, "retrieval.rerankTopK", 5).Int()

	mem := memory.Load(ctx, in.SessionId, in.UserId, query, memoryLimit)
	agent := llm.New(ctx)
	steps := make([]model.LoopStep, 0, maxRounds)
	answer := ""

	for round := 1; round <= maxRounds; round++ {
		scratchpad := memory.Scratchpad(steps)

		// ① 思考：Orchestrator 决定下一步动作（fanout / answer）
		action, thought, err := think(ctx, agent, query, mem.HistoryText(), scratchpad)
		if err != nil {
			thought = "LLM 不可用，使用规则默认 fan-out 全部子 Agent"
			action = model.OrchestratorAction{
				Action: "fanout", SubAgents: []string{"vector", "text", "fs", "db"},
				Reason: "mock 默认",
			}
		}
		step := model.LoopStep{Round: round, Thought: thought, Action: action.Action, SubAgents: action.SubAgents}

		if action.Action == "answer" && action.Answer != "" {
			// Mock 模式下忽略模型给出的空泛答复，交由 Writer 基于真实证据生成。
			if agent.Enabled() {
				answer = action.Answer
			}
			if answer == "" {
				answer = write(ctx, agent, query, mem.HistoryText(), lastReranked(steps))
			}
			steps = append(steps, step)
			break
		}

		// ② Fan-out：并行执行选中的子 Agent
		obs := fanOut(ctx, action.SubAgents, query, scratchpad)
		step.Observations = obs

		// ③ 把所有命中合并重排，作为 Writer 的素材
		allHits := make([]model.Hit, 0)
		for _, o := range obs {
			allHits = append(allHits, o.Hits...)
		}
		reranked := retrieval.Rerank(query, allHits, rerankTopK)
		step.Observations = append(step.Observations, model.SubAgentResult{
			Agent: "reranker", Action: "rerank",
			Observation: fmt.Sprintf("合并 %d 条后重排保留 %d 条", len(allHits), len(reranked)),
			Hits:        reranked,
		})
		steps = append(steps, step)

		// ④ Writer：基于重排后的证据生成答复（最后一轮强制生成）
		if round == maxRounds || len(reranked) > 0 {
			answer = write(ctx, agent, query, mem.HistoryText(), reranked)
		}
	}

	if answer == "" {
		answer = "未能基于检索证据生成回答，请补充相关资料后重试。"
	}
	mem.Save(ctx, query, answer)
	return &model.ChatResult{
		SessionId: mem.SessionId(),
		Answer:    answer,
		Rounds:    len(steps),
		Steps:     steps,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// think 让 Orchestrator 决策下一步动作。
func think(ctx context.Context, agent *llm.Client, query, history, scratchpad string) (model.OrchestratorAction, string, error) {
	system := `你是协同办公系统中的 Orchestrator Agent，处于 Agent 循环中。
根据用户问题、历史对话和已有观察，决定下一步动作。只输出 JSON：
{"action":"fanout|answer","subAgents":["vector","text","fs","db"],"answer":"最终答复","reason":"决策理由"}
- 信息不足时 action=fanout，选择需要的子 Agent（vector=向量库, text=文本库, fs=文件系统, db=数据库）
- 已有足够证据时 action=answer，给出最终答复
- subAgents 只在 fanout 时填写；answer 只在 action=answer 时填写`
	user := fmt.Sprintf("历史对话：\n%s\n\n已有观察：\n%s\n\n当前问题：%s", history, scratchpad, query)
	var action model.OrchestratorAction
	if err := agent.ChatJSON(ctx, system, user, &action); err != nil {
		return model.OrchestratorAction{}, "", err
	}
	return action, action.Reason, nil
}

// fanOut 并行执行选中的子 Agent。
func fanOut(ctx context.Context, names []string, query, scratchpad string) []model.SubAgentResult {
	if len(names) == 0 {
		return nil
	}
	results := make([]model.SubAgentResult, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		sa := subagent.GetByName(name)
		if sa == nil {
			continue
		}
		wg.Add(1)
		go func(idx int, agent subagent.SubAgent) {
			defer wg.Done()
			r, err := agent.Run(ctx, query, scratchpad)
			if err != nil {
				r = model.SubAgentResult{Agent: agent.Name(), Observation: err.Error()}
			}
			results[idx] = r
		}(i, sa)
	}
	wg.Wait()
	out := make([]model.SubAgentResult, 0, len(results))
	for _, r := range results {
		if r.Agent != "" {
			out = append(out, r)
		}
	}
	return out
}

// write 基于重排后的证据生成最终答复。
func write(ctx context.Context, agent *llm.Client, query, history string, hits []model.Hit) string {
	if !agent.Enabled() {
		return mockAnswer(query, hits)
	}
	var b strings.Builder
	b.WriteString("历史对话：\n")
	b.WriteString(history)
	b.WriteString("\n用户问题：")
	b.WriteString(query)
	b.WriteString("\n\n检索证据（已重排）：\n")
	for _, h := range hits {
		b.WriteString(fmt.Sprintf("[%s] %s 章节:%s 片段:%s\n", h.Source, h.Title, h.Heading, truncate(h.Content+" "+h.Row, 160)))
	}
	answer, err := agent.Chat(ctx,
		"你是 Writer Agent。只能依据给定证据回答问题，证据不足时明确说明，不要编造。引用来源用 [来源:引用标识] 标注。",
		b.String())
	if err != nil {
		return mockAnswer(query, hits)
	}
	return answer
}

func mockAnswer(query string, hits []model.Hit) string {
	if len(hits) == 0 {
		return fmt.Sprintf("针对「%s」，多路检索未找到相关证据。\n建议补充知识库文档、文件或数据库表后重试。", query)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("针对「%s」，多路检索并重排后得到 %d 条证据：\n\n", query, len(hits)))
	for i, h := range hits {
		heading := h.Heading
		if heading == "" {
			heading = "正文"
		}
		loc := h.Title
		if h.FilePath != "" {
			loc = h.FilePath
		}
		if h.TableName != "" {
			loc = h.TableName
		}
		b.WriteString(fmt.Sprintf("%d. [%s] %s（章节:%s，重排分 %.3f）\n   %s\n",
			i+1, h.Source, loc, heading, h.RerankScore, truncate(h.Content+" "+h.Row, 140)))
	}
	return b.String()
}

// lastReranked 从历史步骤中取出最后一次重排后的命中结果。
func lastReranked(steps []model.LoopStep) []model.Hit {
	for i := len(steps) - 1; i >= 0; i-- {
		for _, o := range steps[i].Observations {
			if o.Agent == "reranker" {
				return o.Hits
			}
		}
	}
	return nil
}

func truncate(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "..."
}
