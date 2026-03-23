// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"github.com/darren-you/auth_service/template_server/internal/logic"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func MeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewMeLogic(r.Context(), svcCtx)
		resp, err := l.Me()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
