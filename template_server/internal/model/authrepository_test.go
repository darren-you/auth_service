package model

import (
	"database/sql"
	"testing"
	"time"
)

func TestFromTenantRecordAllowsNullTimestamps(t *testing.T) {
	record := &AuthTenants{
		Id:        5,
		TenantKey: "elook",
		Name:      "Elook",
		Enabled:   1,
		CreatedAt: sql.NullTime{},
		UpdatedAt: sql.NullTime{},
	}

	tenant := fromTenantRecord(record)
	if tenant == nil {
		t.Fatal("expected tenant to be returned")
	}
	if !tenant.CreatedAt.IsZero() {
		t.Fatalf("expected zero created_at, got %v", tenant.CreatedAt)
	}
	if !tenant.UpdatedAt.IsZero() {
		t.Fatalf("expected zero updated_at, got %v", tenant.UpdatedAt)
	}
}

func TestFromProviderRecordAllowsNullTimestamps(t *testing.T) {
	record := &AuthProviderConfigs{
		Id:         8,
		TenantId:   5,
		Provider:   "wechat",
		ClientType: "miniprogram",
		Enabled:    1,
		AppId:      "wx123",
		CreatedAt:  sql.NullTime{},
		UpdatedAt:  sql.NullTime{},
	}

	provider := fromProviderRecord(record)
	if provider == nil {
		t.Fatal("expected provider to be returned")
	}
	if !provider.CreatedAt.IsZero() {
		t.Fatalf("expected zero created_at, got %v", provider.CreatedAt)
	}
	if !provider.UpdatedAt.IsZero() {
		t.Fatalf("expected zero updated_at, got %v", provider.UpdatedAt)
	}
}

func TestFromSessionRecordAllowsNullTimestamps(t *testing.T) {
	expiresAt := time.Date(2026, time.March, 23, 13, 0, 0, 0, time.UTC)
	record := &AuthSessions{
		Id:               9,
		TenantId:         1,
		AuthUserId:       2,
		Provider:         "phone",
		ClientType:       "app",
		RefreshTokenHash: "hash",
		ExpiresAt:        expiresAt,
		RevokedAt:        sql.NullTime{},
		LastSeenAt:       sql.NullTime{},
		MetadataJson:     `{"token_user_id":12}`,
		CreatedAt:        sql.NullTime{},
		UpdatedAt:        sql.NullTime{},
	}

	session := fromSessionRecord(record)
	if session == nil {
		t.Fatal("expected session to be returned")
	}
	if !session.CreatedAt.IsZero() {
		t.Fatalf("expected zero created_at, got %v", session.CreatedAt)
	}
	if !session.UpdatedAt.IsZero() {
		t.Fatalf("expected zero updated_at, got %v", session.UpdatedAt)
	}
	if !session.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expires_at %v, got %v", expiresAt, session.ExpiresAt)
	}
}

func TestFromUserRecordAllowsNullTimestamps(t *testing.T) {
	record := &AuthUsers{
		Id:          13,
		TenantId:    2,
		DisplayName: "DarrenYou",
		AvatarUrl:   "https://example.com/avatar.png",
		Role:        "user",
		Status:      "active",
		LastLoginAt: sql.NullTime{},
		CreatedAt:   sql.NullTime{},
		UpdatedAt:   sql.NullTime{},
	}

	user := fromUserRecord(record)
	if user == nil {
		t.Fatal("expected user to be returned")
	}
	if !user.CreatedAt.IsZero() {
		t.Fatalf("expected zero created_at, got %v", user.CreatedAt)
	}
	if !user.UpdatedAt.IsZero() {
		t.Fatalf("expected zero updated_at, got %v", user.UpdatedAt)
	}
}

func TestFromIdentityRecordAllowsNullTimestamps(t *testing.T) {
	record := &AuthIdentities{
		Id:              13,
		TenantId:        2,
		AuthUserId:      13,
		Provider:        "wechat",
		ClientType:      "web",
		ProviderSubject: "openid",
		UnionId:         "unionid",
		ProfileJson:     "{}",
		CreatedAt:       sql.NullTime{},
		UpdatedAt:       sql.NullTime{},
	}

	identity := fromIdentityRecord(record)
	if identity == nil {
		t.Fatal("expected identity to be returned")
	}
	if !identity.CreatedAt.IsZero() {
		t.Fatalf("expected zero created_at, got %v", identity.CreatedAt)
	}
	if !identity.UpdatedAt.IsZero() {
		t.Fatalf("expected zero updated_at, got %v", identity.UpdatedAt)
	}
}

func TestAuthTimestampSchemaNeedsRepair(t *testing.T) {
	canonical := []authTimestampColumnState{
		{TableName: "auth_tenants", ColumnName: "created_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}},
		{TableName: "auth_tenants", ColumnName: "updated_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}, Extra: "on update CURRENT_TIMESTAMP"},
		{TableName: "auth_provider_configs", ColumnName: "created_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}},
		{TableName: "auth_provider_configs", ColumnName: "updated_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}, Extra: "on update CURRENT_TIMESTAMP"},
		{TableName: "auth_users", ColumnName: "created_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}},
		{TableName: "auth_users", ColumnName: "updated_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}, Extra: "on update CURRENT_TIMESTAMP"},
	}
	if authTimestampSchemaNeedsRepair(canonical, "auth_tenants") {
		t.Fatal("expected canonical auth_tenants schema to skip repair")
	}
	if authTimestampSchemaNeedsRepair(canonical, "auth_provider_configs") {
		t.Fatal("expected canonical auth_provider_configs schema to skip repair")
	}
	if authTimestampSchemaNeedsRepair(canonical, "auth_users") {
		t.Fatal("expected canonical schema to skip repair")
	}

	drifted := []authTimestampColumnState{
		{TableName: "auth_tenants", ColumnName: "created_at", IsNullable: "YES", ColumnDefault: sql.NullString{}},
		{TableName: "auth_tenants", ColumnName: "updated_at", IsNullable: "YES", ColumnDefault: sql.NullString{}, Extra: ""},
		{TableName: "auth_provider_configs", ColumnName: "created_at", IsNullable: "YES", ColumnDefault: sql.NullString{}},
		{TableName: "auth_provider_configs", ColumnName: "updated_at", IsNullable: "YES", ColumnDefault: sql.NullString{}, Extra: ""},
		{TableName: "auth_users", ColumnName: "created_at", IsNullable: "YES", ColumnDefault: sql.NullString{}},
		{TableName: "auth_users", ColumnName: "updated_at", IsNullable: "YES", ColumnDefault: sql.NullString{}, Extra: ""},
	}
	if !authTimestampSchemaNeedsRepair(drifted, "auth_tenants") {
		t.Fatal("expected drifted auth_tenants schema to require repair")
	}
	if !authTimestampSchemaNeedsRepair(drifted, "auth_provider_configs") {
		t.Fatal("expected drifted auth_provider_configs schema to require repair")
	}
	if !authTimestampSchemaNeedsRepair(drifted, "auth_users") {
		t.Fatal("expected drifted schema to require repair")
	}
}
