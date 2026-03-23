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
