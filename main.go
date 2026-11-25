package main

import (
	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"

	"ali-agent/internal/bootstrap"
	_ "ali-agent/internal/logic"
	"ali-agent/internal/utility/router"
)

func main() {
	ctx := gctx.New()
	if err := bootstrap.Init(ctx); err != nil {
		glog.Fatalf(ctx, "bootstrap init failed: %v", err)
	}
	s := g.Server()
	router.Register(s)
	glog.Infof(ctx, "ali-agent starting")
	s.Run()
}
