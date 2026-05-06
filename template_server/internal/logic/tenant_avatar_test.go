package logic

import (
	"testing"

	"github.com/darren-you/auth_service/template_server/internal/config"
)

func TestResolveTenantAvatarURL(t *testing.T) {
	const defaultAvatarURL = "https://files.xdarren.com/elook/images/defaults/avatar.jpg"
	const previousDefaultAvatarURL = "https://files.xdarren.com/elook/images/defaults/avatar.png"
	const customAvatarURL = "https://files.xdarren.com/elook/images/useravator/custom.jpg"

	cfg := config.Config{
		Auth: config.AuthConfig{
			Tenants: []config.TenantConfig{
				{
					Key:              "elook",
					DefaultAvatarURL: defaultAvatarURL,
					LegacyDefaultAvatarURLs: []string{
						previousDefaultAvatarURL,
					},
				},
				{
					Key: "stellar",
				},
			},
		},
	}

	tests := []struct {
		name      string
		tenantKey string
		avatarURL string
		want      string
	}{
		{
			name:      "empty avatar uses tenant default",
			tenantKey: "elook",
			avatarURL: " ",
			want:      defaultAvatarURL,
		},
		{
			name:      "legacy default avatar uses tenant default",
			tenantKey: "elook",
			avatarURL: previousDefaultAvatarURL,
			want:      defaultAvatarURL,
		},
		{
			name:      "custom avatar is preserved",
			tenantKey: "elook",
			avatarURL: customAvatarURL,
			want:      customAvatarURL,
		},
		{
			name:      "tenant without default preserves empty avatar",
			tenantKey: "stellar",
			avatarURL: " ",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveTenantAvatarURL(cfg, tt.tenantKey, tt.avatarURL); got != tt.want {
				t.Fatalf("resolveTenantAvatarURL(%q, %q) = %q, want %q", tt.tenantKey, tt.avatarURL, got, tt.want)
			}
		})
	}
}
