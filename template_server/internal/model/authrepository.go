package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type AuthRepository interface {
	SyncCatalog(ctx context.Context, tenantConfigs []config.TenantConfig) error
	FindTenantByKey(ctx context.Context, tenantKey string) (*AuthTenant, error)
	FindTenantByID(ctx context.Context, tenantID uint) (*AuthTenant, error)
	FindProviderConfig(ctx context.Context, tenantID uint, provider string, clientType string) (*AuthProviderConfig, error)
	FindUserByIdentity(ctx context.Context, tenantID uint, provider string, subject string) (*AuthUser, *AuthIdentity, error)
	CreateUserWithIdentity(ctx context.Context, user *AuthUser, identity *AuthIdentity) error
	UpdateUserLogin(ctx context.Context, userID uint, displayName string, avatarURL string, lastLoginAt time.Time) error
	UpdateIdentity(ctx context.Context, identityID uint, clientType string, unionID string, profileJSON string) error
	FindUserByID(ctx context.Context, userID uint) (*AuthUser, error)
	CreateSession(ctx context.Context, authSession *AuthSession) error
	FindSessionByHash(ctx context.Context, refreshTokenHash string) (*AuthSession, error)
	RevokeSessionByHash(ctx context.Context, refreshTokenHash string, revokedAt time.Time) error
}

type authRepository struct {
	conn            sqlx.SqlConn
	tenants         AuthTenantsModel
	providerConfigs AuthProviderConfigsModel
	users           AuthUsersModel
	identities      AuthIdentitiesModel
	sessions        AuthSessionsModel
}

func NewAuthRepository(conn sqlx.SqlConn) AuthRepository {
	return &authRepository{
		conn:            conn,
		tenants:         NewAuthTenantsModel(conn),
		providerConfigs: NewAuthProviderConfigsModel(conn),
		users:           NewAuthUsersModel(conn),
		identities:      NewAuthIdentitiesModel(conn),
		sessions:        NewAuthSessionsModel(conn),
	}
}

func (r *authRepository) SyncCatalog(ctx context.Context, tenantConfigs []config.TenantConfig) error {
	for _, tenantCfg := range tenantConfigs {
		tenant, err := r.FindTenantByKey(ctx, tenantCfg.Key)
		if err != nil {
			return err
		}

		if tenant == nil {
			record := toTenantRecord(&AuthTenant{
				TenantKey: tenantCfg.Key,
				Name:      tenantCfg.Name,
				Enabled:   tenantCfg.Enabled,
			})
			ret, err := r.tenants.Insert(ctx, record)
			if err != nil {
				return err
			}
			id, err := ret.LastInsertId()
			if err != nil {
				return err
			}
			tenant = &AuthTenant{
				ID:        uint(id),
				TenantKey: record.TenantKey,
				Name:      record.Name,
				Enabled:   record.Enabled == 1,
			}
		} else {
			tenant.Name = strings.TrimSpace(tenantCfg.Name)
			tenant.Enabled = tenantCfg.Enabled
			if err := r.tenants.Update(ctx, toTenantRecord(tenant)); err != nil {
				return err
			}
		}

		for _, providerCfg := range tenantCfg.Providers {
			provider, err := r.FindProviderConfig(ctx, tenant.ID, providerCfg.Provider, providerCfg.ClientType)
			if err != nil {
				return err
			}

			if provider == nil {
				_, err := r.providerConfigs.Insert(ctx, toProviderRecord(&AuthProviderConfig{
					TenantID:       tenant.ID,
					Provider:       providerCfg.Provider,
					ClientType:     providerCfg.ClientType,
					Enabled:        providerCfg.Enabled,
					AppID:          providerCfg.AppID,
					AppSecret:      providerCfg.AppSecret,
					RedirectURI:    providerCfg.RedirectURI,
					Scope:          providerCfg.Scope,
					TeamID:         providerCfg.TeamID,
					ClientID:       providerCfg.ClientID,
					KeyID:          providerCfg.KeyID,
					SigningKey:     providerCfg.SigningKey,
					TestPhone:      providerCfg.TestPhone,
					TestCaptcha:    providerCfg.TestCaptcha,
					TestCaptchaKey: providerCfg.TestCaptchaKey,
					ExtraJSON:      providerCfg.ExtraJSON,
				}))
				if err != nil {
					return err
				}
				continue
			}

			provider.Enabled = providerCfg.Enabled
			provider.AppID = providerCfg.AppID
			provider.AppSecret = providerCfg.AppSecret
			provider.RedirectURI = providerCfg.RedirectURI
			provider.Scope = providerCfg.Scope
			provider.TeamID = providerCfg.TeamID
			provider.ClientID = providerCfg.ClientID
			provider.KeyID = providerCfg.KeyID
			provider.SigningKey = providerCfg.SigningKey
			provider.TestPhone = providerCfg.TestPhone
			provider.TestCaptcha = providerCfg.TestCaptcha
			provider.TestCaptchaKey = providerCfg.TestCaptchaKey
			provider.ExtraJSON = providerCfg.ExtraJSON
			if err := r.providerConfigs.Update(ctx, toProviderRecord(provider)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *authRepository) FindTenantByKey(ctx context.Context, tenantKey string) (*AuthTenant, error) {
	record, err := r.tenants.FindOneByTenantKey(ctx, normalizeRecordKey(tenantKey))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fromTenantRecord(record), nil
}

func (r *authRepository) FindTenantByID(ctx context.Context, tenantID uint) (*AuthTenant, error) {
	record, err := r.tenants.FindOne(ctx, uint64(tenantID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fromTenantRecord(record), nil
}

func (r *authRepository) FindProviderConfig(ctx context.Context, tenantID uint, provider string, clientType string) (*AuthProviderConfig, error) {
	record, err := r.providerConfigs.FindOneByTenantIdProviderClientType(ctx, uint64(tenantID), normalizeRecordKey(provider), normalizeRecordKey(clientType))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fromProviderRecord(record), nil
}

func (r *authRepository) FindUserByIdentity(ctx context.Context, tenantID uint, provider string, subject string) (*AuthUser, *AuthIdentity, error) {
	identityRecord, err := r.identities.FindOneByTenantIdProviderProviderSubject(ctx, uint64(tenantID), normalizeRecordKey(provider), strings.TrimSpace(subject))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	userRecord, err := r.users.FindOne(ctx, identityRecord.AuthUserId)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	return fromUserRecord(userRecord), fromIdentityRecord(identityRecord), nil
}

func (r *authRepository) CreateUserWithIdentity(ctx context.Context, user *AuthUser, identity *AuthIdentity) error {
	return r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		users := r.users.withSession(session)
		identities := r.identities.withSession(session)

		userRecord := toUserRecord(user)
		ret, err := users.Insert(ctx, userRecord)
		if err != nil {
			return err
		}
		userID, err := ret.LastInsertId()
		if err != nil {
			return err
		}

		user.ID = uint(userID)
		identity.AuthUserID = user.ID

		identityRecord := toIdentityRecord(identity)
		ret, err = identities.Insert(ctx, identityRecord)
		if err != nil {
			return err
		}
		identityID, err := ret.LastInsertId()
		if err != nil {
			return err
		}
		identity.ID = uint(identityID)

		return nil
	})
}

func (r *authRepository) UpdateUserLogin(ctx context.Context, userID uint, displayName string, avatarURL string, lastLoginAt time.Time) error {
	_, err := r.conn.ExecCtx(ctx,
		"update auth_users set display_name = ?, avatar_url = ?, last_login_at = ?, updated_at = now() where id = ?",
		strings.TrimSpace(displayName),
		strings.TrimSpace(avatarURL),
		lastLoginAt,
		userID,
	)
	return err
}

func (r *authRepository) UpdateIdentity(ctx context.Context, identityID uint, clientType string, unionID string, profileJSON string) error {
	_, err := r.conn.ExecCtx(ctx,
		"update auth_identities set client_type = ?, union_id = ?, profile_json = ?, updated_at = now() where id = ?",
		normalizeRecordKey(clientType),
		strings.TrimSpace(unionID),
		profileJSON,
		identityID,
	)
	return err
}

func (r *authRepository) FindUserByID(ctx context.Context, userID uint) (*AuthUser, error) {
	record, err := r.users.FindOne(ctx, uint64(userID))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fromUserRecord(record), nil
}

func (r *authRepository) CreateSession(ctx context.Context, authSession *AuthSession) error {
	ret, err := r.sessions.Insert(ctx, toSessionRecord(authSession))
	if err != nil {
		return err
	}
	id, err := ret.LastInsertId()
	if err != nil {
		return err
	}
	authSession.ID = uint(id)
	return nil
}

func (r *authRepository) FindSessionByHash(ctx context.Context, refreshTokenHash string) (*AuthSession, error) {
	record, err := r.sessions.FindOneByRefreshTokenHash(ctx, strings.TrimSpace(refreshTokenHash))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return fromSessionRecord(record), nil
}

func (r *authRepository) RevokeSessionByHash(ctx context.Context, refreshTokenHash string, revokedAt time.Time) error {
	_, err := r.conn.ExecCtx(ctx,
		"update auth_sessions set revoked_at = ?, last_seen_at = ?, updated_at = now() where refresh_token_hash = ? and revoked_at is null",
		revokedAt,
		revokedAt,
		strings.TrimSpace(refreshTokenHash),
	)
	return err
}

func fromTenantRecord(record *AuthTenants) *AuthTenant {
	if record == nil {
		return nil
	}
	return &AuthTenant{
		ID:        uint(record.Id),
		TenantKey: record.TenantKey,
		Name:      record.Name,
		Enabled:   record.Enabled == 1,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func toTenantRecord(record *AuthTenant) *AuthTenants {
	if record == nil {
		return nil
	}
	return &AuthTenants{
		Id:        uint64(record.ID),
		TenantKey: normalizeRecordKey(record.TenantKey),
		Name:      strings.TrimSpace(record.Name),
		Enabled:   boolToInt64(record.Enabled),
	}
}

func fromProviderRecord(record *AuthProviderConfigs) *AuthProviderConfig {
	if record == nil {
		return nil
	}
	return &AuthProviderConfig{
		ID:             uint(record.Id),
		TenantID:       uint(record.TenantId),
		Provider:       record.Provider,
		ClientType:     record.ClientType,
		Enabled:        record.Enabled == 1,
		AppID:          record.AppId,
		AppSecret:      record.AppSecret,
		RedirectURI:    record.RedirectUri,
		Scope:          record.Scope,
		TeamID:         record.TeamId,
		ClientID:       record.ClientId,
		KeyID:          record.KeyId,
		SigningKey:     record.SigningKey,
		TestPhone:      record.TestPhone,
		TestCaptcha:    record.TestCaptcha,
		TestCaptchaKey: record.TestCaptchaKey,
		ExtraJSON:      record.ExtraJson,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}
}

func toProviderRecord(record *AuthProviderConfig) *AuthProviderConfigs {
	if record == nil {
		return nil
	}
	return &AuthProviderConfigs{
		Id:             uint64(record.ID),
		TenantId:       uint64(record.TenantID),
		Provider:       normalizeRecordKey(record.Provider),
		ClientType:     normalizeRecordKey(record.ClientType),
		Enabled:        boolToInt64(record.Enabled),
		AppId:          strings.TrimSpace(record.AppID),
		AppSecret:      strings.TrimSpace(record.AppSecret),
		RedirectUri:    strings.TrimSpace(record.RedirectURI),
		Scope:          strings.TrimSpace(record.Scope),
		TeamId:         strings.TrimSpace(record.TeamID),
		ClientId:       strings.TrimSpace(record.ClientID),
		KeyId:          strings.TrimSpace(record.KeyID),
		SigningKey:     strings.TrimSpace(record.SigningKey),
		TestPhone:      strings.TrimSpace(record.TestPhone),
		TestCaptcha:    strings.TrimSpace(record.TestCaptcha),
		TestCaptchaKey: strings.TrimSpace(record.TestCaptchaKey),
		ExtraJson:      strings.TrimSpace(record.ExtraJSON),
	}
}

func fromUserRecord(record *AuthUsers) *AuthUser {
	if record == nil {
		return nil
	}
	var lastLoginAt *time.Time
	if record.LastLoginAt.Valid {
		lastLogin := record.LastLoginAt.Time
		lastLoginAt = &lastLogin
	}
	return &AuthUser{
		ID:          uint(record.Id),
		TenantID:    uint(record.TenantId),
		DisplayName: record.DisplayName,
		AvatarURL:   record.AvatarUrl,
		Role:        record.Role,
		Status:      record.Status,
		LastLoginAt: lastLoginAt,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func toUserRecord(record *AuthUser) *AuthUsers {
	if record == nil {
		return nil
	}
	lastLogin := sql.NullTime{}
	if record.LastLoginAt != nil {
		lastLogin = sql.NullTime{
			Time:  *record.LastLoginAt,
			Valid: true,
		}
	}
	return &AuthUsers{
		Id:          uint64(record.ID),
		TenantId:    uint64(record.TenantID),
		DisplayName: strings.TrimSpace(record.DisplayName),
		AvatarUrl:   strings.TrimSpace(record.AvatarURL),
		Role:        strings.TrimSpace(record.Role),
		Status:      strings.TrimSpace(record.Status),
		LastLoginAt: lastLogin,
	}
}

func fromIdentityRecord(record *AuthIdentities) *AuthIdentity {
	if record == nil {
		return nil
	}
	return &AuthIdentity{
		ID:              uint(record.Id),
		TenantID:        uint(record.TenantId),
		AuthUserID:      uint(record.AuthUserId),
		Provider:        record.Provider,
		ClientType:      record.ClientType,
		ProviderSubject: record.ProviderSubject,
		UnionID:         record.UnionId,
		ProfileJSON:     record.ProfileJson,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func toIdentityRecord(record *AuthIdentity) *AuthIdentities {
	if record == nil {
		return nil
	}
	return &AuthIdentities{
		Id:              uint64(record.ID),
		TenantId:        uint64(record.TenantID),
		AuthUserId:      uint64(record.AuthUserID),
		Provider:        normalizeRecordKey(record.Provider),
		ClientType:      normalizeRecordKey(record.ClientType),
		ProviderSubject: strings.TrimSpace(record.ProviderSubject),
		UnionId:         strings.TrimSpace(record.UnionID),
		ProfileJson:     record.ProfileJSON,
	}
}

func fromSessionRecord(record *AuthSessions) *AuthSession {
	if record == nil {
		return nil
	}
	var revokedAt *time.Time
	if record.RevokedAt.Valid {
		value := record.RevokedAt.Time
		revokedAt = &value
	}
	var lastSeenAt *time.Time
	if record.LastSeenAt.Valid {
		value := record.LastSeenAt.Time
		lastSeenAt = &value
	}
	return &AuthSession{
		ID:               uint(record.Id),
		TenantID:         uint(record.TenantId),
		AuthUserID:       uint(record.AuthUserId),
		Provider:         record.Provider,
		ClientType:       record.ClientType,
		RefreshTokenHash: record.RefreshTokenHash,
		ExpiresAt:        record.ExpiresAt,
		RevokedAt:        revokedAt,
		LastSeenAt:       lastSeenAt,
		MetadataJSON:     record.MetadataJson,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func toSessionRecord(record *AuthSession) *AuthSessions {
	if record == nil {
		return nil
	}
	revokedAt := sql.NullTime{}
	if record.RevokedAt != nil {
		revokedAt = sql.NullTime{Time: *record.RevokedAt, Valid: true}
	}
	lastSeenAt := sql.NullTime{}
	if record.LastSeenAt != nil {
		lastSeenAt = sql.NullTime{Time: *record.LastSeenAt, Valid: true}
	}
	return &AuthSessions{
		Id:               uint64(record.ID),
		TenantId:         uint64(record.TenantID),
		AuthUserId:       uint64(record.AuthUserID),
		Provider:         normalizeRecordKey(record.Provider),
		ClientType:       normalizeRecordKey(record.ClientType),
		RefreshTokenHash: strings.TrimSpace(record.RefreshTokenHash),
		ExpiresAt:        record.ExpiresAt,
		RevokedAt:        revokedAt,
		LastSeenAt:       lastSeenAt,
		MetadataJson:     record.MetadataJSON,
	}
}

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func normalizeRecordKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
