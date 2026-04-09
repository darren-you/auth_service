package model

import "time"

type AuthTenant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantKey string    `gorm:"column:tenant_key;size:64;uniqueIndex" json:"tenant_key"`
	Name      string    `gorm:"size:191" json:"name"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AuthTenant) TableName() string {
	return "auth_tenants"
}

type AuthProviderConfig struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TenantID       uint      `gorm:"uniqueIndex:idx_tenant_provider_client,priority:1;index" json:"tenant_id"`
	Provider       string    `gorm:"size:32;uniqueIndex:idx_tenant_provider_client,priority:2" json:"provider"`
	ClientType     string    `gorm:"column:client_type;size:32;uniqueIndex:idx_tenant_provider_client,priority:3" json:"client_type"`
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	AppID          string    `gorm:"column:app_id;size:191" json:"app_id"`
	AppSecret      string    `gorm:"column:app_secret;type:text" json:"app_secret"`
	RedirectURI    string    `gorm:"column:redirect_uri;type:text" json:"redirect_uri"`
	Scope          string    `gorm:"size:128" json:"scope"`
	TeamID         string    `gorm:"column:team_id;size:64" json:"team_id"`
	ClientID       string    `gorm:"column:client_id;size:191" json:"client_id"`
	KeyID          string    `gorm:"column:key_id;size:64" json:"key_id"`
	SigningKey     string    `gorm:"column:signing_key;type:longtext" json:"signing_key"`
	TestPhone      string    `gorm:"column:test_phone;size:32" json:"test_phone"`
	TestCaptcha    string    `gorm:"column:test_captcha;size:32" json:"test_captcha"`
	TestCaptchaKey string    `gorm:"column:test_captcha_key;size:64" json:"test_captcha_key"`
	ExtraJSON      string    `gorm:"column:extra_json;type:longtext" json:"extra_json"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (AuthProviderConfig) TableName() string {
	return "auth_provider_configs"
}

type AuthUser struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	TenantID    uint       `gorm:"index" json:"tenant_id"`
	TokenUserID uint       `gorm:"column:token_user_id;uniqueIndex:idx_tenant_token_user,priority:2" json:"token_user_id"`
	DisplayName string     `gorm:"column:display_name;size:191" json:"display_name"`
	AvatarURL   string     `gorm:"column:avatar_url;type:text" json:"avatar_url"`
	Role        string     `gorm:"size:32;default:user" json:"role"`
	Status      string     `gorm:"size:32;default:active" json:"status"`
	LastLoginAt *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (AuthUser) TableName() string {
	return "auth_users"
}

type AuthIdentity struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	TenantID        uint      `gorm:"uniqueIndex:idx_tenant_provider_subject,priority:1;index" json:"tenant_id"`
	AuthUserID      uint      `gorm:"column:auth_user_id;index" json:"auth_user_id"`
	Provider        string    `gorm:"size:32;uniqueIndex:idx_tenant_provider_subject,priority:2" json:"provider"`
	ClientType      string    `gorm:"column:client_type;size:32" json:"client_type"`
	ProviderSubject string    `gorm:"column:provider_subject;size:191;uniqueIndex:idx_tenant_provider_subject,priority:3" json:"provider_subject"`
	UnionID         string    `gorm:"column:union_id;size:191" json:"union_id"`
	ProfileJSON     string    `gorm:"column:profile_json;type:longtext" json:"profile_json"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (AuthIdentity) TableName() string {
	return "auth_identities"
}

type AuthSession struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	TenantID         uint       `gorm:"index" json:"tenant_id"`
	AuthUserID       uint       `gorm:"column:auth_user_id;index" json:"auth_user_id"`
	Provider         string     `gorm:"size:32" json:"provider"`
	ClientType       string     `gorm:"column:client_type;size:32" json:"client_type"`
	RefreshTokenHash string     `gorm:"column:refresh_token_hash;size:64;uniqueIndex" json:"refresh_token_hash"`
	ExpiresAt        time.Time  `gorm:"column:expires_at;index" json:"expires_at"`
	RevokedAt        *time.Time `gorm:"column:revoked_at" json:"revoked_at"`
	LastSeenAt       *time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
	MetadataJSON     string     `gorm:"column:metadata_json;type:longtext" json:"metadata_json"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (AuthSession) TableName() string {
	return "auth_sessions"
}
