package health

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"ali-agent/api/v1"
	"ali-agent/internal/logic/subagent"
)

type Controller struct{}

func New() *Controller { return &Controller{} }

func (c *Controller) Health(ctx context.Context, req *v1.HealthReq) (*v1.HealthRes, error) {
	dbStatus := "down"
	if err := g.DB().PingMaster(); err == nil {
		dbStatus = "up"
	}
	llmMode := "mock"
	if g.Cfg().MustGet(ctx, "llm.apiKey", "").String() != "" {
		llmMode = "api"
	}
	names := make([]string, 0)
	for _, a := range subagent.All() {
		names = append(names, a.Name())
	}
	return &v1.HealthRes{
		Status:   "ok",
		Database: dbStatus,
		LlmMode:  llmMode,
		Agents:   names,
	}, nil
}
