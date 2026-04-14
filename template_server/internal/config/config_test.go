package config

import (
	"strings"
	"testing"
)

func TestValidateAcceptsContainerNetworkBridgeBaseURL(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:           "elook",
					Name:          "Elook",
					BridgeBaseURL: "http://elook_server_prod:8080",
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected container-network bridge_base_url to pass validation, got %v", err)
	}
}

func TestValidateRejectsHostIPBridgeBaseURL(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:           "elook",
					Name:          "Elook",
					BridgeBaseURL: "http://124.221.158.155:8098",
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected host-ip bridge_base_url to fail validation")
	}
	if !strings.Contains(err.Error(), "bridge_base_url") {
		t.Fatalf("expected bridge_base_url validation error, got %v", err)
	}
}
