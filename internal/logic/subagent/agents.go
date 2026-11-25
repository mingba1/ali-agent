package subagent

import (
	"context"
	"fmt"

	"ali-agent/internal/logic/retrieval"
	"ali-agent/internal/model"
	"ali-agent/internal/utility/llm"
)

// SubAgent 子 Agent 接口：接收用户问题与草稿板，决定工具调用并返回观察结果。
type SubAgent interface {
	Name() string
	Run(ctx context.Context, query string, scratchpad string) (model.SubAgentResult, error)
}

// All 返回所有可用子 Agent。
func All() []SubAgent {
	return []SubAgent{
		&VectorAgent{},
		&TextAgent{},
		&FileAgent{},
		&DBAgent{},
	}
}

// GetByName 按名称获取子 Agent。
func GetByName(name string) SubAgent {
	for _, a := range All() {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// decide 让子 Agent 决定调用哪个工具及参数。
func decide(ctx context.Context, agent *llm.Client, system, query, scratchpad string) (model.SubAgentDecision, error) {
	user := fmt.Sprintf("用户问题：%s\n\n已有观察：\n%s", query, scratchpad)
	var d model.SubAgentDecision
	if err := agent.ChatJSON(ctx, system, user, &d); err != nil {
		return model.SubAgentDecision{}, err
	}
	if d.TopK <= 0 {
		d.TopK = 10
	}
	return d, nil
}

// ============ Vector Agent ============

type VectorAgent struct{}

func (a *VectorAgent) Name() string { return "vector" }

func (a *VectorAgent) Run(ctx context.Context, query, scratchpad string) (model.SubAgentResult, error) {
	agent := llm.New(ctx)
	decision, err := decide(ctx, agent,
		"你是向量检索子 Agent。从用户问题中抽取语义查询，调用 vector_search 工具在向量库中召回相关文档分块。只输出 JSON：{\"tool\":\"vector_search\",\"query\":\"查询语句\",\"topK\":10}",
		query, scratchpad)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	hits, err := retrieval.VectorSearch(ctx, decision.Query, decision.TopK)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	return model.SubAgentResult{
		Agent:       a.Name(),
		Action:      decision.Tool,
		Observation: fmt.Sprintf("向量检索召回 %d 条，已重排", len(hits)),
		Hits:        hits,
	}, nil
}

// ============ Text Agent ============

type TextAgent struct{}

func (a *TextAgent) Name() string { return "text" }

func (a *TextAgent) Run(ctx context.Context, query, scratchpad string) (model.SubAgentResult, error) {
	agent := llm.New(ctx)
	decision, err := decide(ctx, agent,
		"你是文本检索子 Agent。从用户问题中抽取关键词，调用 text_search 工具在文本库中做关键词匹配。只输出 JSON：{\"tool\":\"text_search\",\"query\":\"查询语句\",\"topK\":10}",
		query, scratchpad)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	hits, err := retrieval.TextSearch(ctx, decision.Query, decision.TopK)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	return model.SubAgentResult{
		Agent:       a.Name(),
		Action:      decision.Tool,
		Observation: fmt.Sprintf("文本检索召回 %d 条", len(hits)),
		Hits:        hits,
	}, nil
}

// ============ File Agent ============

type FileAgent struct{}

func (a *FileAgent) Name() string { return "fs" }

func (a *FileAgent) Run(ctx context.Context, query, scratchpad string) (model.SubAgentResult, error) {
	agent := llm.New(ctx)
	decision, err := decide(ctx, agent,
		"你是文件检索子 Agent。从用户问题中抽取关键词，调用 grep 工具在文件系统中搜索代码或文档。只输出 JSON：{\"tool\":\"grep\",\"query\":\"空格分隔的关键词\",\"topK\":20}",
		query, scratchpad)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	hits, err := retrieval.FileSearch(ctx, decision.Query, decision.TopK)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	return model.SubAgentResult{
		Agent:       a.Name(),
		Action:      decision.Tool,
		Observation: fmt.Sprintf("文件系统检索到 %d 处匹配", len(hits)),
		Hits:        hits,
	}, nil
}

// ============ DB Agent ============

type DBAgent struct{}

func (a *DBAgent) Name() string { return "db" }

func (a *DBAgent) Run(ctx context.Context, query, scratchpad string) (model.SubAgentResult, error) {
	agent := llm.New(ctx)
	decision, err := decide(ctx, agent,
		"你是数据库子 Agent。根据用户问题生成只读 SQL（SELECT），调用 sql 工具查询 OLTP/OLAP 数据库。只输出 JSON：{\"tool\":\"sql\",\"query\":\"SELECT ...\",\"topK\":30}",
		query, scratchpad)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: err.Error()}, nil
	}
	hits, err := retrieval.DBSearch(ctx, decision.Query, decision.TopK)
	if err != nil {
		return model.SubAgentResult{Agent: a.Name(), Observation: "SQL 执行失败: " + err.Error()}, nil
	}
	return model.SubAgentResult{
		Agent:       a.Name(),
		Action:      decision.Tool,
		Observation: fmt.Sprintf("数据库查询返回 %d 行", len(hits)),
		Hits:        hits,
	}, nil
}
