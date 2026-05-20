package config

import (
	"strings"
	"testing"

	"github.com/darren-you/auth_service/providerkeys"
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

func TestValidateAcceptsTenantDefaultAvatarURL(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:              "elook",
					Name:             "Elook",
					DefaultAvatarURL: "https://files.xdarren.com/elook/images/defaults/avatar.jpg",
					LegacyDefaultAvatarURLs: []string{
						"https://files.xdarren.com/elook/images/defaults/avatar.png",
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default avatar urls to pass validation, got %v", err)
	}
}

func TestValidateRejectsInvalidTenantDefaultAvatarURL(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:              "elook",
					Name:             "Elook",
					DefaultAvatarURL: "files.xdarren.com/elook/images/defaults/avatar.jpg",
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected invalid default_avatar_url to fail validation")
	}
	if !strings.Contains(err.Error(), "default_avatar_url") {
		t.Fatalf("expected default_avatar_url validation error, got %v", err)
	}
}

func TestValidateAcceptsFirebaseAuthProviderWithProjectID(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:  "tinytext",
					Name: "TinyText",
					Providers: []ProviderConfig{
						{
							Provider:   providerkeys.ProviderFirebaseAuth,
							ClientType: providerkeys.ClientTypeWeb,
							ClientID:   "tinytext-global",
						},
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected firebase_auth provider with project id to pass validation, got %v", err)
	}
}

func TestValidateAcceptsFirebaseAuthProviderWithExtraJSONProjectID(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:  "tinytext",
					Name: "TinyText",
					Providers: []ProviderConfig{
						{
							Provider:   providerkeys.ProviderFirebaseAuth,
							ClientType: providerkeys.ClientTypeWeb,
							ExtraJSON:  `{"project_id":"tinytext-global"}`,
						},
					},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected firebase_auth provider with extra_json.project_id to pass validation, got %v", err)
	}
}

func TestValidateRejectsFirebaseAuthProviderWithoutProjectID(t *testing.T) {
	cfg := Config{
		JWT: JWTConfig{
			Secret: "test-secret",
		},
		Auth: AuthConfig{
			Tenants: []TenantConfig{
				{
					Key:  "tinytext",
					Name: "TinyText",
					Providers: []ProviderConfig{
						{
							Provider:   providerkeys.ProviderFirebaseAuth,
							ClientType: providerkeys.ClientTypeWeb,
						},
					},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected firebase_auth provider without project id to fail validation")
	}
	if !strings.Contains(err.Error(), "firebase_auth") {
		t.Fatalf("expected firebase_auth validation error, got %v", err)
	}
}
