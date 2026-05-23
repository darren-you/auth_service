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

func (s *authRepoStub) UpsertIdentityForUser(context.Context, *model.AuthUser, *model.AuthIdentity) error {
	panic("unexpected call to UpsertIdentityForUser")
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

type phoneIdentityAuthRepoStub struct {
	authRepoStub
	identityOwner      *model.AuthUser
	identity           *model.AuthIdentity
	userByToken        *model.AuthUser
	createdUser        *model.AuthUser
	createdIdentity    *model.AuthIdentity
	upsertedUserID     uint
	upsertedIdentity   *model.AuthIdentity
	updatedLoginUserID uint
	updatedTokenUserID uint
	updatedTokenAuthID uint
}

func (s *phoneIdentityAuthRepoStub) FindUserByIdentity(context.Context, uint, string, string) (*model.AuthUser, *model.AuthIdentity, error) {
	return s.identityOwner, s.identity, nil
}

func (s *phoneIdentityAuthRepoStub) FindUserByTokenUserID(context.Context, uint, uint) (*model.AuthUser, error) {
	return s.userByToken, nil
}

func (s *phoneIdentityAuthRepoStub) CreateUserWithIdentity(_ context.Context, user *model.AuthUser, identity *model.AuthIdentity) error {
	user.ID = 30
	identity.ID = 40
	identity.AuthUserID = user.ID
	s.createdUser = user
	s.createdIdentity = identity
	return nil
}

func (s *phoneIdentityAuthRepoStub) UpsertIdentityForUser(_ context.Context, user *model.AuthUser, identity *model.AuthIdentity) error {
	s.upsertedUserID = user.ID
	copied := *identity
	copied.AuthUserID = user.ID
	s.upsertedIdentity = &copied
	return nil
}

func (s *phoneIdentityAuthRepoStub) UpdateUserLogin(_ context.Context, userID uint, _ string, _ string, _ time.Time) error {
	s.updatedLoginUserID = userID
	return nil
}

func (s *phoneIdentityAuthRepoStub) UpdateUserTokenUserID(_ context.Context, authUserID uint, tokenUserID uint) error {
	s.updatedTokenAuthID = authUserID
	s.updatedTokenUserID = tokenUserID
	return nil
}

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

func TestUpsertPhoneIdentityUserBindsIdentityToExistingTokenUser(t *testing.T) {
	t.Parallel()

	repo := &phoneIdentityAuthRepoStub{
		identityOwner: &model.AuthUser{ID: 20, TenantID: 7},
		identity:      &model.AuthIdentity{ID: 21, TenantID: 7, AuthUserID: 20, Provider: "phone", ProviderSubject: "17608265580"},
		userByToken:   &model.AuthUser{ID: 10, TenantID: 7, TokenUserID: 8, DisplayName: "微信用户"},
	}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{AuthRepo: repo})

	user, err := authFlow.upsertPhoneIdentityUser(
		&model.AuthTenant{ID: 7, TenantKey: "elook"},
		&model.AuthProviderConfig{ClientType: providerkeys.ClientTypeMiniProgram},
		"17608265580",
		"微信用户",
		"https://example.com/avatar.jpg",
		8,
		`{"phone":"17608265580"}`,
	)
	if err != nil {
		t.Fatalf("upsertPhoneIdentityUser returned error: %v", err)
	}
	if user.ID != 10 {
		t.Fatalf("expected existing token auth user 10, got %d", user.ID)
	}
	if repo.upsertedUserID != 10 {
		t.Fatalf("expected identity to be upserted for auth user 10, got %d", repo.upsertedUserID)
	}
	if repo.upsertedIdentity == nil || repo.upsertedIdentity.ID != 21 || repo.upsertedIdentity.AuthUserID != 10 {
		t.Fatalf("expected existing phone identity to move to auth user 10, got %#v", repo.upsertedIdentity)
	}
	if repo.updatedTokenAuthID != 0 || repo.updatedTokenUserID != 0 {
		t.Fatalf("did not expect token user id update, got auth=%d token=%d", repo.updatedTokenAuthID, repo.updatedTokenUserID)
	}
}

func TestUpsertPhoneIdentityUserRejectsIdentityBoundToAnotherTokenUser(t *testing.T) {
	t.Parallel()

	repo := &phoneIdentityAuthRepoStub{
		identityOwner: &model.AuthUser{ID: 20, TenantID: 7, TokenUserID: 9},
		identity:      &model.AuthIdentity{ID: 21, TenantID: 7, AuthUserID: 20, Provider: "phone", ProviderSubject: "17608265580"},
		userByToken:   &model.AuthUser{ID: 10, TenantID: 7, TokenUserID: 8, DisplayName: "微信用户"},
	}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{AuthRepo: repo})

	_, err := authFlow.upsertPhoneIdentityUser(
		&model.AuthTenant{ID: 7, TenantKey: "elook"},
		&model.AuthProviderConfig{ClientType: providerkeys.ClientTypeMiniProgram},
		"17608265580",
		"微信用户",
		"",
		8,
		`{"phone":"17608265580"}`,
	)
	if err == nil || !strings.Contains(err.Error(), "phone already bound to another account") {
		t.Fatalf("expected phone already bound error, got %v", err)
	}
}

func TestUpsertPhoneIdentityUserCreatesTokenMappedUserWhenMissing(t *testing.T) {
	t.Parallel()

	repo := &phoneIdentityAuthRepoStub{}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{AuthRepo: repo})

	user, err := authFlow.upsertPhoneIdentityUser(
		&model.AuthTenant{ID: 7, TenantKey: "elook"},
		&model.AuthProviderConfig{ClientType: providerkeys.ClientTypeMiniProgram},
		"17608265580",
		"微信用户",
		"",
		8,
		`{"phone":"17608265580"}`,
	)
	if err != nil {
		t.Fatalf("upsertPhoneIdentityUser returned error: %v", err)
	}
	if user.ID != 30 || repo.createdUser == nil || repo.createdUser.TokenUserID != 8 {
		t.Fatalf("expected created token-mapped user, got user=%#v created=%#v", user, repo.createdUser)
	}
	if repo.createdIdentity == nil || repo.createdIdentity.AuthUserID != 30 || repo.createdIdentity.ProviderSubject != "17608265580" {
		t.Fatalf("expected created phone identity, got %#v", repo.createdIdentity)
	}
}

func TestBindProviderIdentityToBusinessUserMovesIdentityToExistingTokenUser(t *testing.T) {
	t.Parallel()

	repo := &phoneIdentityAuthRepoStub{
		identityOwner: &model.AuthUser{ID: 20, TenantID: 7, TokenUserID: 1},
		identity: &model.AuthIdentity{
			ID:              21,
			TenantID:        7,
			AuthUserID:      20,
			Provider:        providerkeys.ProviderWeChatMiniProgram,
			ClientType:      providerkeys.ClientTypeMiniProgram,
			ProviderSubject: "openid-123",
			UnionID:         "union-123",
		},
		userByToken: &model.AuthUser{ID: 10, TenantID: 7, TokenUserID: 17, DisplayName: "旧昵称"},
	}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{AuthRepo: repo})

	user, err := authFlow.bindProviderIdentityToBusinessUser(
		&model.AuthTenant{ID: 7, TenantKey: "elook"},
		&model.AuthProviderConfig{ClientType: providerkeys.ClientTypeMiniProgram},
		providerkeys.ProviderWeChatMiniProgram,
		"openid-123",
		"union-123",
		"微信用户",
		"https://example.com/avatar.jpg",
		`{"openid":"openid-123"}`,
		repo.identityOwner,
		17,
	)
	if err != nil {
		t.Fatalf("bindProviderIdentityToBusinessUser returned error: %v", err)
	}
	if user.ID != 10 {
		t.Fatalf("expected existing token auth user 10, got %d", user.ID)
	}
	if repo.upsertedUserID != 10 {
		t.Fatalf("expected identity to be upserted for auth user 10, got %d", repo.upsertedUserID)
	}
	if repo.upsertedIdentity == nil || repo.upsertedIdentity.AuthUserID != 10 || repo.upsertedIdentity.ID != 21 {
		t.Fatalf("expected provider identity to move to auth user 10, got %#v", repo.upsertedIdentity)
	}
	if repo.updatedLoginUserID != 10 {
		t.Fatalf("expected token auth user login to update, got %d", repo.updatedLoginUserID)
	}
	if repo.updatedTokenAuthID != 0 || repo.updatedTokenUserID != 0 {
		t.Fatalf("did not expect token user id update, got auth=%d token=%d", repo.updatedTokenAuthID, repo.updatedTokenUserID)
	}
}

func TestBindProviderIdentityToBusinessUserAssignsTokenUserWhenMissing(t *testing.T) {
	t.Parallel()

	repo := &phoneIdentityAuthRepoStub{
		identityOwner: &model.AuthUser{ID: 20, TenantID: 7},
		identity: &model.AuthIdentity{
			ID:              21,
			TenantID:        7,
			AuthUserID:      20,
			Provider:        providerkeys.ProviderWeChatMiniProgram,
			ProviderSubject: "openid-123",
		},
	}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{AuthRepo: repo})

	user, err := authFlow.bindProviderIdentityToBusinessUser(
		&model.AuthTenant{ID: 7, TenantKey: "elook"},
		&model.AuthProviderConfig{ClientType: providerkeys.ClientTypeMiniProgram},
		providerkeys.ProviderWeChatMiniProgram,
		"openid-123",
		"union-123",
		"微信用户",
		"",
		`{"openid":"openid-123"}`,
		repo.identityOwner,
		17,
	)
	if err != nil {
		t.Fatalf("bindProviderIdentityToBusinessUser returned error: %v", err)
	}
	if user.ID != 20 || user.TokenUserID != 17 {
		t.Fatalf("expected identity owner 20 to receive token user id 17, got %#v", user)
	}
	if repo.updatedTokenAuthID != 20 || repo.updatedTokenUserID != 17 {
		t.Fatalf("expected token user id update for auth user 20, got auth=%d token=%d", repo.updatedTokenAuthID, repo.updatedTokenUserID)
	}
	if repo.upsertedIdentity != nil {
		t.Fatalf("did not expect identity move when no token auth user exists, got %#v", repo.upsertedIdentity)
	}
}

func TestBindProviderIdentityToBusinessUserRepairsStaleTokenUserWhenMissing(t *testing.T) {
	t.Parallel()

	repo := &phoneIdentityAuthRepoStub{
		identityOwner: &model.AuthUser{ID: 20, TenantID: 7, TokenUserID: 1},
		identity: &model.AuthIdentity{
			ID:              21,
			TenantID:        7,
			AuthUserID:      20,
			Provider:        providerkeys.ProviderWeChatMiniProgram,
			ProviderSubject: "openid-123",
		},
	}
	authFlow := newAuthFlow(context.Background(), &svc.ServiceContext{AuthRepo: repo})

	user, err := authFlow.bindProviderIdentityToBusinessUser(
		&model.AuthTenant{ID: 7, TenantKey: "elook"},
		&model.AuthProviderConfig{ClientType: providerkeys.ClientTypeMiniProgram},
		providerkeys.ProviderWeChatMiniProgram,
		"openid-123",
		"union-123",
		"微信用户",
		"",
		`{"openid":"openid-123"}`,
		repo.identityOwner,
		17,
	)
	if err != nil {
		t.Fatalf("bindProviderIdentityToBusinessUser returned error: %v", err)
	}
	if user.ID != 20 || user.TokenUserID != 17 {
		t.Fatalf("expected identity owner 20 to repair token user id to 17, got %#v", user)
	}
	if repo.updatedTokenAuthID != 20 || repo.updatedTokenUserID != 17 {
		t.Fatalf("expected token user id repair for auth user 20, got auth=%d token=%d", repo.updatedTokenAuthID, repo.updatedTokenUserID)
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
