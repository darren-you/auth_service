package handler

import (
	"net/http"

	"github.com/darren-you/auth_service/template_server/internal/logic"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func InternalUpdateUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.InternalUpdateUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewInternalUpdateUserLogic(r.Context(), svcCtx)
		resp, err := l.InternalUpdateUser(r.Header.Get("X-Auth-Service-Key"), &req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
