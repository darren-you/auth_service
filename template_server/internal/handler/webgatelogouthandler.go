// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"github.com/darren-you/auth_service/template_server/internal/logic"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func WebGateLogoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WebGateLogoutReq
		if err := parseOptionalJSON(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		fillWebGateCommonRequest(r, &req.TenantKey, &req.ClientType)

		l := logic.NewWebGateLogoutLogic(r.Context(), svcCtx)
		resp, err := l.WebGateLogout(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if resp.Cookie != nil {
			http.SetCookie(w, resp.Cookie)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
