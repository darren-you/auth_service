// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/handler"
	"github.com/darren-you/auth_service/template_server/internal/middleware"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/pkg/responsex"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "config/config.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(
		c.Server.RestConf(c.Log.LogConf(c.Server.Name)),
		rest.WithCustomCors(func(header http.Header) {
			if len(c.Server.AllowHeaders) > 0 {
				header.Set("Access-Control-Allow-Headers", strings.Join(c.Server.AllowHeaders, ","))
			}
			if len(c.Server.AllowMethods) > 0 {
				header.Set("Access-Control-Allow-Methods", strings.Join(c.Server.AllowMethods, ","))
			}
		}, nil, c.Server.AllowOrigins...),
	)
	defer server.Stop()

	httpx.SetErrorHandlerCtx(errorx.Handler)
	httpx.SetOkHandler(responsex.OkHandler)
	server.Use(middleware.NewRequestIDMiddleware().Handle)
	server.Use(middleware.NewRequestLogMiddleware().Handle)

	ctx, err := svc.NewServiceContext(c)
	if err != nil {
		panic(err)
	}
	defer ctx.Close()

	if err := ctx.AuthRepo.SyncCatalog(context.Background(), c.Auth.Tenants); err != nil {
		panic(err)
	}
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Server.Host, c.Server.Port)
	server.Start()
}
