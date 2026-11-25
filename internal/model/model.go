package model

import "github.com/gogf/gf/v2/os/gtime"

// ===== 会话与记忆 =====

type ChatSession struct {
	Id        string      `json:"id"        orm:"id"`
	UserId    string      `json:"userId"    orm:"user_id"`
	Title     string      `json:"title"     orm:"title"`
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at"`
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at"`
}

type ChatMessage struct {
	Id        int64       `json:"id"        orm:"id"`
	SessionId string      `json:"sessionId" orm:"session_id"`
	Role      string      `json:"role"      orm:"role"`
	Content   string      `json:"content"   orm:"content"`
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at"`
}

// ===== 文档与分块 =====

type Doc struct {
	Id        int64       `json:"id"        orm:"id"`
	Title     string      `json:"title"     orm:"title"`
	Content   string      `json:"content"   orm:"content"`
	Tags      string      `json:"tags"      orm:"tags"`
	Author    string      `json:"author"    orm:"author"`
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at"`
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at"`
}

type DocChunk struct {
	Id          int64       `json:"id"          orm:"id"`
	DocId       int64       `json:"docId"       orm:"doc_id"`
	ChunkIndex  int         `json:"chunkIndex"  orm:"chunk_index"`
	Heading     string      `json:"heading"     orm:"heading"`
	Content     string      `json:"content"     orm:"content"`
	StartOffset int         `json:"startOffset" orm:"start_offset"`
	EndOffset   int         `json:"endOffset"   orm:"end_offset"`
	Embedding   string      `json:"-"           orm:"embedding"`
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"`
}

// ===== 检索结果 =====

// Hit 单条检索命中，来源可能是 vector/text/fs/db。
type Hit struct {
	Source      string  `json:"source"`      // vector/text/fs/db
	DocId       int64   `json:"docId"`       // 文档ID（vector/text）
	ChunkId     int64   `json:"chunkId"`     // 分块ID（vector/text）
	Title       string  `json:"title"`       // 标题
	Heading     string  `json:"heading"`     // 章节
	Tags        string  `json:"tags"`        // 标签
	Content     string  `json:"content"`     // 内容片段
	FilePath    string  `json:"filePath"`    // 文件路径（fs）
	LineNo      int     `json:"lineNo"`      // 行号（fs）
	TableName   string  `json:"tableName"`   // 表名（db）
	Row         string  `json:"row"`         // 行内容（db）
	Ref         string  `json:"ref"`         // 引用标识
	Score       float64 `json:"score"`       // 原始分
	RerankScore float64 `json:"rerankScore"` // 重排后分
	StartOffset int     `json:"startOffset"` // chunk 在原文中的起始位置
	EndOffset   int     `json:"endOffset"`   // chunk 在原文中的结束位置
}

// SubAgentResult 子 Agent 执行结果。
type SubAgentResult struct {
	Agent       string `json:"agent"`
	Action      string `json:"action"`
	Observation string `json:"observation"`
	Hits        []Hit  `json:"hits"`
}

// ===== Agent Loop =====

// OrchestratorAction Orchestrator 决定的下一步动作。
type OrchestratorAction struct {
	Action    string   `json:"action"`              // fanout | answer | reflect
	SubAgents []string `json:"subAgents,omitempty"` // fanout 时选择的子 Agent
	Answer    string   `json:"answer,omitempty"`    // answer 时的最终答复
	Reason    string   `json:"reason"`              // 决策理由
}

// SubAgentDecision 子 Agent 的决策（调用哪个工具、参数是什么）。
type SubAgentDecision struct {
	Tool  string `json:"tool"`  // vector_search/text_search/grep/sql
	Query string `json:"query"` // 检索/查询语句
	TopK  int    `json:"topK"`  // 返回条数
}

// LoopStep Agent 循环中的一步（思考→行动→观察）。
type LoopStep struct {
	Round        int              `json:"round"`
	Thought      string           `json:"thought"`   // Orchestrator 的思考
	Action       string           `json:"action"`    // fanout/answer
	SubAgents    []string         `json:"subAgents"` // 选中的子 Agent
	Observations []SubAgentResult `json:"observations"`
}

// ChatResult 对话返回。
type ChatResult struct {
	SessionId string     `json:"sessionId"`
	Answer    string     `json:"answer"`
	Rounds    int        `json:"rounds"`
	Steps     []LoopStep `json:"steps"`
	LatencyMs int64      `json:"latencyMs"`
}

// ChatInput 对话入参。
type ChatInput struct {
	SessionId string
	UserId    string
	Query     string
}
