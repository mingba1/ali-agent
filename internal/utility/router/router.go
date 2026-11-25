package router

import (
	"github.com/gogf/gf/v2/net/ghttp"

	"ali-agent/internal/controller/chat"
	"ali-agent/internal/controller/health"
)

// Register 注册路由与中间件。
func Register(s *ghttp.Server) {
	s.Use(ghttp.MiddlewareHandlerResponse, ghttp.MiddlewareCORS)
	chatC := chat.New()
	healthC := health.New()
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Bind(
			chatC,
			healthC,
		)
	})
}
