package chat

import (
	"context"

	"ali-agent/api/v1"
	"ali-agent/internal/model"
	"ali-agent/internal/service"
)

type Controller struct{}

func New() *Controller { return &Controller{} }

func (c *Controller) Chat(ctx context.Context, req *v1.ChatReq) (*v1.ChatRes, error) {
	out, err := service.Chat().Chat(ctx, model.ChatInput{
		Query:     req.Query,
		SessionId: req.SessionId,
		UserId:    req.UserId,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ChatRes{
		SessionId: out.SessionId,
		Answer:    out.Answer,
		Rounds:    out.Rounds,
		Steps:     out.Steps,
		LatencyMs: out.LatencyMs,
	}, nil
}
