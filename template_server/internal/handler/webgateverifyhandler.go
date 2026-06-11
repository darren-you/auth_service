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

func WebGateVerifyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := types.WebGateVerifyReq{}
		fillWebGateCommonRequest(r, &req.TenantKey, &req.ClientType)

		l := logic.NewWebGateVerifyLogic(r.Context(), svcCtx)
		if err := l.WebGateVerify(&req, r.Cookies()); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
