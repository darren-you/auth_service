package mysql

import (
	"context"
	stdErrors "errors"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/darren-you/auth_service/template_server/internal/model"
	"gorm.io/gorm"
)

type AuthRepository interface {
	SyncCatalog(ctx context.Context, tenantConfigs []config.TenantConfig) error
	FindTenantByKey(ctx context.Context, tenantKey string) (*model.AuthTenant, error)
	FindTenantByID(ctx context.Context, tenantID uint) (*model.AuthTenant, error)
	FindProviderConfig(ctx context.Context, tenantID uint, provider string, clientType string) (*model.AuthProviderConfig, error)
	FindUserByIdentity(ctx context.Context, tenantID uint, provider string, subject string) (*model.AuthUser, *model.AuthIdentity, error)
	CreateUserWithIdentity(ctx context.Context, user *model.AuthUser, identity *model.AuthIdentity) error
	UpdateUserLogin(ctx context.Context, userID uint, displayName string, avatarURL string, lastLoginAt time.Time) error
	UpdateIdentity(ctx context.Context, identityID uint, clientType string, unionID string, profileJSON string) error
	FindUserByID(ctx context.Context, userID uint) (*model.AuthUser, error)
	CreateSession(ctx context.Context, authSession *model.AuthSession) error
	FindSessionByHash(ctx context.Context, refreshTokenHash string) (*model.AuthSession, error)
	RevokeSessionByHash(ctx context.Context, refreshTokenHash string, revokedAt time.Time) error
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) SyncCatalog(ctx context.Context, tenantConfigs []config.TenantConfig) error {
	for _, tenantCfg := range tenantConfigs {
		var tenant model.AuthTenant
		err := r.db.WithContext(ctx).
			Where("tenant_key = ?", tenantCfg.Key).
			First(&tenant).Error
		if err != nil && !stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			tenant = model.AuthTenant{
				TenantKey: tenantCfg.Key,
			}
		}
		tenant.Name = tenantCfg.Name
		tenant.Enabled = tenantCfg.Enabled

		if err := r.db.WithContext(ctx).Save(&tenant).Error; err != nil {
			return err
		}

		for _, providerCfg := range tenantCfg.Providers {
			var record model.AuthProviderConfig
			err := r.db.WithContext(ctx).
				Where("tenant_id = ? AND provider = ? AND client_type = ?", tenant.ID, providerCfg.Provider, providerCfg.ClientType).
				First(&record).Error
			if err != nil && !stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				record = model.AuthProviderConfig{
					TenantID:   tenant.ID,
					Provider:   providerCfg.Provider,
					ClientType: providerCfg.ClientType,
				}
			}

			record.Enabled = providerCfg.Enabled
			record.AppID = providerCfg.AppID
			record.AppSecret = providerCfg.AppSecret
			record.RedirectURI = providerCfg.RedirectURI
			record.Scope = providerCfg.Scope
			record.TeamID = providerCfg.TeamID
			record.ClientID = providerCfg.ClientID
			record.KeyID = providerCfg.KeyID
			record.SigningKey = providerCfg.SigningKey
			record.TestPhone = providerCfg.TestPhone
			record.TestCaptcha = providerCfg.TestCaptcha
			record.TestCaptchaKey = providerCfg.TestCaptchaKey
			record.ExtraJSON = providerCfg.ExtraJSON

			if err := r.db.WithContext(ctx).Save(&record).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *authRepository) FindTenantByKey(ctx context.Context, tenantKey string) (*model.AuthTenant, error) {
	var tenant model.AuthTenant
	if err := r.db.WithContext(ctx).Where("tenant_key = ?", tenantKey).First(&tenant).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}

func (r *authRepository) FindTenantByID(ctx context.Context, tenantID uint) (*model.AuthTenant, error) {
	var tenant model.AuthTenant
	if err := r.db.WithContext(ctx).First(&tenant, tenantID).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}

func (r *authRepository) FindProviderConfig(ctx context.Context, tenantID uint, provider string, clientType string) (*model.AuthProviderConfig, error) {
	var providerConfig model.AuthProviderConfig
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND provider = ? AND client_type = ?", tenantID, provider, clientType).
		First(&providerConfig).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &providerConfig, nil
}

func (r *authRepository) FindUserByIdentity(ctx context.Context, tenantID uint, provider string, subject string) (*model.AuthUser, *model.AuthIdentity, error) {
	var identity model.AuthIdentity
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND provider = ? AND provider_subject = ?", tenantID, provider, subject).
		First(&identity).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	var user model.AuthUser
	if err := r.db.WithContext(ctx).First(&user, identity.AuthUserID).Error; err != nil {
		return nil, nil, err
	}
	return &user, &identity, nil
}

func (r *authRepository) CreateUserWithIdentity(ctx context.Context, user *model.AuthUser, identity *model.AuthIdentity) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		identity.AuthUserID = user.ID
		return tx.Create(identity).Error
	})
}

func (r *authRepository) UpdateUserLogin(ctx context.Context, userID uint, displayName string, avatarURL string, lastLoginAt time.Time) error {
	updates := map[string]interface{}{
		"display_name":  displayName,
		"avatar_url":    avatarURL,
		"last_login_at": lastLoginAt,
	}
	return r.db.WithContext(ctx).Model(&model.AuthUser{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *authRepository) UpdateIdentity(ctx context.Context, identityID uint, clientType string, unionID string, profileJSON string) error {
	updates := map[string]interface{}{
		"client_type":  clientType,
		"union_id":     unionID,
		"profile_json": profileJSON,
	}
	return r.db.WithContext(ctx).Model(&model.AuthIdentity{}).Where("id = ?", identityID).Updates(updates).Error
}

func (r *authRepository) FindUserByID(ctx context.Context, userID uint) (*model.AuthUser, error) {
	var user model.AuthUser
	if err := r.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) CreateSession(ctx context.Context, authSession *model.AuthSession) error {
	return r.db.WithContext(ctx).Create(authSession).Error
}

func (r *authRepository) FindSessionByHash(ctx context.Context, refreshTokenHash string) (*model.AuthSession, error) {
	var authSession model.AuthSession
	if err := r.db.WithContext(ctx).
		Where("refresh_token_hash = ?", refreshTokenHash).
		First(&authSession).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &authSession, nil
}

func (r *authRepository) RevokeSessionByHash(ctx context.Context, refreshTokenHash string, revokedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.AuthSession{}).
		Where("refresh_token_hash = ? AND revoked_at IS NULL", refreshTokenHash).
		Updates(map[string]interface{}{
			"revoked_at":   revokedAt,
			"last_seen_at": revokedAt,
		}).Error
}
