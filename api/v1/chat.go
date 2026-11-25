package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/internal/model"
)

type ChatReq struct {
	g.Meta    `path:"/chat" method:"post" tags:"Chat" summary:"Orchestrator Agent Loop 问答"`
	Query     string `json:"query"     v:"required#问题不能为空"`
	SessionId string `json:"sessionId" dc:"会话ID，不传则自动创建"`
	UserId    string `json:"userId"    d:"default"`
}

type ChatRes struct {
	SessionId string           `json:"sessionId"`
	Answer    string           `json:"answer"`
	Rounds    int              `json:"rounds"`
	Steps     []model.LoopStep `json:"steps"`
	LatencyMs int64            `json:"latencyMs"`
}

type HealthReq struct {
	g.Meta `path:"/health" method:"get" tags:"Health" summary:"健康检查"`
}

type HealthRes struct {
	Status   string   `json:"status"`
	Database string   `json:"database"`
	LlmMode  string   `json:"llmMode"`
	Agents   []string `json:"agents"`
}
