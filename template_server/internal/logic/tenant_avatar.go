package logic

import (
	"strings"

	"github.com/darren-you/auth_service/template_server/internal/config"
)

func resolveTenantAvatarURL(cfg config.Config, tenantKey string, avatarURL string) string {
	trimmedAvatarURL := strings.TrimSpace(avatarURL)
	avatarCfg := resolveTenantAvatarConfig(cfg.Auth.Tenants, tenantKey)
	if avatarCfg.defaultAvatarURL == "" {
		return trimmedAvatarURL
	}
	if trimmedAvatarURL == "" || avatarCfg.legacyDefaultAvatarURLs[trimmedAvatarURL] {
		return avatarCfg.defaultAvatarURL
	}
	return trimmedAvatarURL
}

type tenantAvatarConfig struct {
	defaultAvatarURL        string
	legacyDefaultAvatarURLs map[string]bool
}

func resolveTenantAvatarConfig(tenants []config.TenantConfig, tenantKey string) tenantAvatarConfig {
	normalizedTenantKey := strings.ToLower(strings.TrimSpace(tenantKey))
	for _, tenant := range tenants {
		if strings.ToLower(strings.TrimSpace(tenant.Key)) != normalizedTenantKey {
			continue
		}
		defaultAvatarURL := strings.TrimSpace(tenant.DefaultAvatarURL)
		legacyDefaultAvatarURLs := make(map[string]bool, len(tenant.LegacyDefaultAvatarURLs)+1)
		for _, legacyAvatarURL := range tenant.LegacyDefaultAvatarURLs {
			if normalizedLegacyAvatarURL := strings.TrimSpace(legacyAvatarURL); normalizedLegacyAvatarURL != "" {
				legacyDefaultAvatarURLs[normalizedLegacyAvatarURL] = true
			}
		}
		return tenantAvatarConfig{
			defaultAvatarURL:        defaultAvatarURL,
			legacyDefaultAvatarURLs: legacyDefaultAvatarURLs,
		}
	}

	return tenantAvatarConfig{
		legacyDefaultAvatarURLs: map[string]bool{},
	}
}
