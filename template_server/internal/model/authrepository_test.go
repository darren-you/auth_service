package model

import (
	"database/sql"
	"testing"
	"time"
)

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
		{TableName: "auth_users", ColumnName: "created_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}},
		{TableName: "auth_users", ColumnName: "updated_at", IsNullable: "NO", ColumnDefault: sql.NullString{String: "CURRENT_TIMESTAMP", Valid: true}, Extra: "on update CURRENT_TIMESTAMP"},
	}
	if authTimestampSchemaNeedsRepair(canonical, "auth_users") {
		t.Fatal("expected canonical schema to skip repair")
	}

	drifted := []authTimestampColumnState{
		{TableName: "auth_users", ColumnName: "created_at", IsNullable: "YES", ColumnDefault: sql.NullString{}},
		{TableName: "auth_users", ColumnName: "updated_at", IsNullable: "YES", ColumnDefault: sql.NullString{}, Extra: ""},
	}
	if !authTimestampSchemaNeedsRepair(drifted, "auth_users") {
		t.Fatal("expected drifted schema to require repair")
	}
}
