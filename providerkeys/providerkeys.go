package providerkeys

import "strings"

const (
	ProviderWeChatApp         = "wechat_app"
	ProviderWeChatWeb         = "wechat_web"
	ProviderWeChatMiniProgram = "wechat_miniprogram"
	ProviderApple             = "apple"
	ProviderPassword          = "password"
	ProviderPhone             = "phone"
	ProviderGuest             = "guest"
)

const (
	ClientTypeApp         = "app"
	ClientTypeWeb         = "web"
	ClientTypeMiniProgram = "miniprogram"
	ClientTypeIOS         = "ios"
	ClientTypeDefault     = "default"
)

func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func NormalizeClientType(clientType string) string {
	return strings.ToLower(strings.TrimSpace(clientType))
}

func IsWeChatProvider(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderWeChatApp, ProviderWeChatWeb, ProviderWeChatMiniProgram:
		return true
	default:
		return false
	}
}

func WeChatClientType(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderWeChatApp:
		return ClientTypeApp
	case ProviderWeChatWeb:
		return ClientTypeWeb
	case ProviderWeChatMiniProgram:
		return ClientTypeMiniProgram
	default:
		return ""
	}
}
