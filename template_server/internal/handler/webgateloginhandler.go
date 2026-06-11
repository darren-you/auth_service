// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/darren-you/auth_service/template_server/internal/logic"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

const (
	webGateTenantKeyHeader  = "X-Web-Gate-Tenant-Key"
	webGateTenantHeader     = "X-Web-Gate-Tenant"
	webGateClientTypeHeader = "X-Web-Gate-Client-Type"
	webGatePasswordHeader   = "X-Web-Gate-Password"
)

func WebGateLoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WebGateLoginReq
		if err := parseOptionalJSON(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		fillWebGateCommonRequest(r, &req.TenantKey, &req.ClientType)
		if password := firstNonEmptyHeader(r, webGatePasswordHeader); password != "" {
			req.Password = password
		}

		l := logic.NewWebGateLoginLogic(r.Context(), svcCtx)
		resp, err := l.WebGateLogin(&req)
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

func parseOptionalJSON(r *http.Request, target any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func fillWebGateCommonRequest(r *http.Request, tenantKey *string, clientType *string) {
	if value := strings.TrimSpace(r.URL.Query().Get("tenant_key")); value != "" {
		*tenantKey = value
	}
	if value := strings.TrimSpace(r.URL.Query().Get("client_type")); value != "" {
		*clientType = value
	}
	if value := firstNonEmptyHeader(r, webGateTenantKeyHeader, webGateTenantHeader); value != "" {
		*tenantKey = value
	}
	if value := firstNonEmptyHeader(r, webGateClientTypeHeader); value != "" {
		*clientType = value
	}
}

func firstNonEmptyHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}
