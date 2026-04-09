package logic

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/darren-you/auth_service/providerkeys"
	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"github.com/darren-you/auth_service/template_server/internal/store"
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

func (s *authRepoStub) FindUserByTokenUserID(context.Context, uint, uint) (*model.AuthUser, error) {
	panic("unexpected call to FindUserByTokenUserID")
}

func (s *authRepoStub) CreateUserWithIdentity(context.Context, *model.AuthUser, *model.AuthIdentity) error {
	panic("unexpected call to CreateUserWithIdentity")
}

func (s *authRepoStub) UpdateUserLogin(context.Context, uint, string, string, time.Time) error {
	panic("unexpected call to UpdateUserLogin")
}

func (s *authRepoStub) UpdateUserTokenUserID(context.Context, uint, uint) error {
	panic("unexpected call to UpdateUserTokenUserID")
}

func (s *authRepoStub) UpdateUserProfileAndActiveSessions(context.Context, uint, string, string, string, string) error {
	panic("unexpected call to UpdateUserProfileAndActiveSessions")
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

func (s *authRepoStub) FindLatestActiveSessionByTokenUserID(context.Context, uint, uint) (*model.AuthSession, error) {
	panic("unexpected call to FindLatestActiveSessionByTokenUserID")
}

func (s *authRepoStub) RevokeSessionByHash(context.Context, string, time.Time) error {
	panic("unexpected call to RevokeSessionByHash")
}

type kvStoreStub struct {
	setIfAbsentCalls int
	lastKey          string
}

func (s *kvStoreStub) SetIfAbsent(context.Context, string, string, time.Duration) (bool, error) {
	s.setIfAbsentCalls++
	return true, nil
}

func (s *kvStoreStub) Consume(context.Context, string) (bool, error) {
	panic("unexpected call to Consume")
}

func (s *kvStoreStub) Set(context.Context, string, string, time.Duration) error {
	panic("unexpected call to Set")
}

func (s *kvStoreStub) Get(context.Context, string) (string, error) {
	panic("unexpected call to Get")
}

func (s *kvStoreStub) Delete(context.Context, string) error {
	panic("unexpected call to Delete")
}

func (s *kvStoreStub) Exists(context.Context, string) (bool, error) {
	panic("unexpected call to Exists")
}

var _ store.KVStore = (*kvStoreStub)(nil)

func TestGetLoginURLForMiniProgramDoesNotAllocateState(t *testing.T) {
	t.Parallel()

	kv := &kvStoreStub{}
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
				Provider:   providerkeys.ProviderWeChatMiniProgram,
				ClientType: providerkeys.ClientTypeMiniProgram,
				Enabled:    true,
			},
		},
		KVStore: kv,
	})

	resp, err := authFlow.GetLoginURL(&types.GetLoginURLReq{
		Provider:   providerkeys.ProviderWeChatMiniProgram,
		TenantKey:  "demo",
		ClientType: providerkeys.ClientTypeMiniProgram,
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
	if kv.setIfAbsentCalls != 0 {
		t.Fatalf("expected no state allocation, got %d calls", kv.setIfAbsentCalls)
	}
}

func TestGetLoginURLForWeChatAppReturnsStateOnly(t *testing.T) {
	t.Parallel()

	kv := &kvStoreStub{}
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
				Provider:   providerkeys.ProviderWeChatApp,
				ClientType: providerkeys.ClientTypeApp,
				Enabled:    true,
			},
		},
		KVStore: kv,
	})

	resp, err := authFlow.GetLoginURL(&types.GetLoginURLReq{
		Provider:   providerkeys.ProviderWeChatApp,
		TenantKey:  "demo",
		ClientType: providerkeys.ClientTypeApp,
	})
	if err != nil {
		t.Fatalf("GetLoginURL returned error: %v", err)
	}
	if resp.LoginURL != "" {
		t.Fatalf("expected empty login_url, got %q", resp.LoginURL)
	}
	if strings.TrimSpace(resp.State) == "" {
		t.Fatal("expected non-empty state")
	}
	if kv.setIfAbsentCalls != 1 {
		t.Fatalf("expected 1 state allocation, got %d", kv.setIfAbsentCalls)
	}
}

func TestGetLoginURLForWeChatWebBuildsOAuthURL(t *testing.T) {
	t.Parallel()

	kv := &kvStoreStub{}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{
		AuthRepo: &authRepoStub{
			tenant: &model.AuthTenant{
				ID:        1,
				TenantKey: "demo",
				Name:      "Demo",
				Enabled:   true,
			},
			providerConfig: &model.AuthProviderConfig{
				ID:          1,
				TenantID:    1,
				Provider:    providerkeys.ProviderWeChatWeb,
				ClientType:  providerkeys.ClientTypeWeb,
				Enabled:     true,
				AppID:       "wx-app-id",
				RedirectURI: "https://example.com/wechat/callback",
				Scope:       "snsapi_login",
			},
		},
		KVStore: kv,
	})

	resp, err := authFlow.GetLoginURL(&types.GetLoginURLReq{
		Provider:   providerkeys.ProviderWeChatWeb,
		TenantKey:  "demo",
		ClientType: providerkeys.ClientTypeWeb,
	})
	if err != nil {
		t.Fatalf("GetLoginURL returned error: %v", err)
	}
	if strings.TrimSpace(resp.LoginURL) == "" {
		t.Fatal("expected non-empty login_url")
	}
	parsed, err := url.Parse(resp.LoginURL)
	if err != nil {
		t.Fatalf("parse login_url failed: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("appid"); got != "wx-app-id" {
		t.Fatalf("unexpected appid: %s", got)
	}
	if got := query.Get("redirect_uri"); got != "https://example.com/wechat/callback" {
		t.Fatalf("unexpected redirect_uri: %s", got)
	}
	if got := query.Get("state"); got != resp.State {
		t.Fatalf("expected state %q in login_url, got %q", resp.State, got)
	}
}
