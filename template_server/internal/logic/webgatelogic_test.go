package logic

import (
	"context"
	"net/http"
	"testing"

	"github.com/darren-you/auth_service/providerkeys"
	"github.com/darren-you/auth_service/template_server/internal/config"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
)

func TestWebGateLoginSignsCookieAndVerifyAcceptsIt(t *testing.T) {
	svcCtx := newWebGateTestServiceContext()

	login := NewWebGateLoginLogic(context.Background(), svcCtx)
	resp, err := login.WebGateLogin(&types.WebGateLoginReq{
		TenantKey:  "appbox",
		ClientType: "web",
		Password:   "appbox-secret",
	})
	if err != nil {
		t.Fatalf("WebGateLogin returned error: %v", err)
	}
	if resp.Cookie == nil {
		t.Fatalf("expected login cookie")
	}
	if resp.Cookie.Name != "appbox_gate" {
		t.Fatalf("cookie name = %s, want appbox_gate", resp.Cookie.Name)
	}
	if !resp.Cookie.HttpOnly || !resp.Cookie.Secure {
		t.Fatalf("cookie should be HttpOnly and Secure: %#v", resp.Cookie)
	}

	verify := NewWebGateVerifyLogic(context.Background(), svcCtx)
	err = verify.WebGateVerify(
		&types.WebGateVerifyReq{TenantKey: "appbox", ClientType: "web"},
		[]*http.Cookie{resp.Cookie},
	)
	if err != nil {
		t.Fatalf("WebGateVerify returned error: %v", err)
	}
}

func TestWebGateLoginRejectsWrongPassword(t *testing.T) {
	svcCtx := newWebGateTestServiceContext()

	login := NewWebGateLoginLogic(context.Background(), svcCtx)
	_, err := login.WebGateLogin(&types.WebGateLoginReq{
		TenantKey:  "appbox",
		ClientType: "web",
		Password:   "wrong-secret",
	})
	if !appErrors.Is(err, appErrors.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestWebGateVerifyRejectsTenantMismatch(t *testing.T) {
	svcCtx := newWebGateTestServiceContext()

	login := NewWebGateLoginLogic(context.Background(), svcCtx)
	resp, err := login.WebGateLogin(&types.WebGateLoginReq{
		TenantKey:  "appbox",
		ClientType: "web",
		Password:   "appbox-secret",
	})
	if err != nil {
		t.Fatalf("WebGateLogin returned error: %v", err)
	}

	verify := NewWebGateVerifyLogic(context.Background(), svcCtx)
	err = verify.WebGateVerify(
		&types.WebGateVerifyReq{TenantKey: "protocol", ClientType: "web"},
		[]*http.Cookie{resp.Cookie},
	)
	if !appErrors.Is(err, appErrors.ErrUnauthorized) {
		t.Fatalf("error = %v, want ErrUnauthorized", err)
	}
}

func newWebGateTestServiceContext() *svc.ServiceContext {
	return &svc.ServiceContext{
		Config: config.Config{
			JWT: config.JWTConfig{
				Secret: "test-secret",
				Issuer: "auth_service",
			},
			Auth: config.AuthConfig{
				Tenants: []config.TenantConfig{
					{
						Key:     "appbox",
						Name:    "AppBox",
						Enabled: true,
						Providers: []config.ProviderConfig{
							{
								Provider:   providerkeys.ProviderWebGate,
								ClientType: "web",
								Enabled:    true,
								AppSecret:  "appbox-secret",
								ExtraJSON:  `{"cookie_name":"appbox_gate","session_ttl_second":3600}`,
							},
						},
					},
					{
						Key:     "protocol",
						Name:    "Protocol",
						Enabled: true,
						Providers: []config.ProviderConfig{
							{
								Provider:   providerkeys.ProviderWebGate,
								ClientType: "web",
								Enabled:    true,
								AppSecret:  "protocol-secret",
								ExtraJSON:  `{"cookie_name":"protocol_gate","session_ttl_second":3600}`,
							},
						},
					},
				},
			},
		},
	}
}
