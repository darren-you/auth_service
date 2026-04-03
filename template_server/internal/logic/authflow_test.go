package logic

import (
	"context"
	"testing"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
)

type authRepoStub struct {
	tenant         *model.AuthTenant
	providerConfig *model.AuthProviderConfig
}

func (s *authRepoStub) SyncCatalog(context.Context, []config.TenantConfig) error {
	panic("unexpected call to SyncCatalog")
}

func (s *authRepoStub) FindTenantByKey(context.Context, string) (*model.AuthTenant, error) {
	return s.tenant, nil
}

func (s *authRepoStub) FindTenantByID(context.Context, uint) (*model.AuthTenant, error) {
	panic("unexpected call to FindTenantByID")
}

func (s *authRepoStub) FindProviderConfig(context.Context, uint, string, string) (*model.AuthProviderConfig, error) {
	return s.providerConfig, nil
}

func (s *authRepoStub) FindUserByIdentity(context.Context, uint, string, string) (*model.AuthUser, *model.AuthIdentity, error) {
	panic("unexpected call to FindUserByIdentity")
}

func (s *authRepoStub) CreateUserWithIdentity(context.Context, *model.AuthUser, *model.AuthIdentity) error {
	panic("unexpected call to CreateUserWithIdentity")
}

func (s *authRepoStub) UpdateUserLogin(context.Context, uint, string, string, time.Time) error {
	panic("unexpected call to UpdateUserLogin")
}

func (s *authRepoStub) UpdateIdentity(context.Context, uint, string, string, string) error {
	panic("unexpected call to UpdateIdentity")
}

func (s *authRepoStub) FindUserByID(context.Context, uint) (*model.AuthUser, error) {
	panic("unexpected call to FindUserByID")
}

func (s *authRepoStub) CreateSession(context.Context, *model.AuthSession) error {
	panic("unexpected call to CreateSession")
}

func (s *authRepoStub) FindSessionByHash(context.Context, string) (*model.AuthSession, error) {
	panic("unexpected call to FindSessionByHash")
}

func (s *authRepoStub) RevokeSessionByHash(context.Context, string, time.Time) error {
	panic("unexpected call to RevokeSessionByHash")
}

func TestGetLoginURLForMiniProgramDoesNotAllocateState(t *testing.T) {
	t.Parallel()

	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{
		AuthRepo: &authRepoStub{
			tenant: &model.AuthTenant{
				ID:        1,
				TenantKey: "demo",
				Name:      "Demo",
				Enabled:   true,
			},
			providerConfig: &model.AuthProviderConfig{
				ID:         1,
				TenantID:   1,
				Provider:   "wechat",
				ClientType: "miniprogram",
				Enabled:    true,
			},
		},
	})

	resp, err := authFlow.GetLoginURL(&types.GetLoginURLReq{
		Provider:   "wechat",
		TenantKey:  "demo",
		ClientType: "miniprogram",
	})
	if err != nil {
		t.Fatalf("GetLoginURL returned error: %v", err)
	}
	if resp.LoginURL != "" {
		t.Fatalf("expected empty login_url, got %q", resp.LoginURL)
	}
	if resp.State != "" {
		t.Fatalf("expected empty state, got %q", resp.State)
	}
}
