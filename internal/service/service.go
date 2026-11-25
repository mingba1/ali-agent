package service

import (
	"context"

	"ali-agent/internal/model"
)

// IChat 对话编排服务（Orchestrator Agent Loop）。
type IChat interface {
	Chat(ctx context.Context, in model.ChatInput) (*model.ChatResult, error)
}

// IDoc 文档与分块服务。
type IDoc interface {
	Reindex(ctx context.Context) (int, int, error)
}

var (
	localChat IChat
	localDoc  IDoc
)

func Chat() IChat { return localChat }
func Doc() IDoc   { return localDoc }

func RegisterChat(i IChat) { localChat = i }
func RegisterDoc(i IDoc)   { localDoc = i }
