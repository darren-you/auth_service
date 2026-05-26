package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/darren-you/auth_service/providerkeys"
	"github.com/darren-you/auth_service/template_server/pkg/session"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	Server ServerConfig `json:"server"`
	MySQL  MySQLConfig  `json:"mysql"`
	Redis  RedisConfig  `json:"redis"`
	Log    LogConfig    `json:"log"`
	JWT    JWTConfig    `json:"jwt"`
	Auth   AuthConfig   `json:"auth"`
}

type ServerConfig struct {
	Name             string               `json:"name,default=auth_service"`
	Mode             string               `json:"mode,default=pro,options=dev|test|rt|pre|pro"`
	Host             string               `json:"host,default=0.0.0.0"`
	Port             int                  `json:"port,default=8080"`
	ReadTimeout      int                  `json:"read_timeout,default=30"`
	WriteTimeout     int                  `json:"write_timeout,default=30"`
	RequestTimeoutMS int64                `json:"timeout,optional"`
	MaxConns         int                  `json:"max_conns,default=10000"`
	MaxBytes         int64                `json:"max_bytes,default=1048576"`
	Middlewares      rest.MiddlewaresConf `json:"middlewares,optional"`
	TraceIgnorePaths []string             `json:"trace_ignore_paths,optional"`
	AllowOrigins     []string             `json:"allow_origins,optional"`
	AllowMethods     []string             `json:"allow_methods,optional"`
	AllowHeaders     []string             `json:"allow_headers,optional"`
}

type MySQLConfig struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password,optional"`
	Database        string `json:"database"`
	MaxOpenConns    int    `json:"max_open_conns,optional"`
	MaxIdleConns    int    `json:"max_idle_conns,optional"`
	ConnMaxLifetime int    `json:"conn_max_lifetime,optional"`
	ConnMaxIdleTime int    `json:"conn_max_idle_time,optional"`
}

type RedisConfig struct {
	Addr           string `json:"addr"`
	Password       string `json:"password,optional"`
	DB             int    `json:"db,optional"`
	PoolSize       int    `json:"pool_size,optional"`
	MinIdleConns   int    `json:"min_idle_conns,optional"`
	DialTimeoutMS  int    `json:"dial_timeout_ms,optional"`
	ReadTimeoutMS  int    `json:"read_timeout_ms,optional"`
	WriteTimeoutMS int    `json:"write_timeout_ms,optional"`
	PoolTimeoutMS  int    `json:"pool_timeout_ms,optional"`
}

type LogConfig struct {
	Level    string `json:"level,default=info,options=debug|info|error|severe"`
	File     string `json:"file,optional"`
	Encoding string `json:"encoding,default=json,options=plain|json"`
}

type JWTConfig struct {
	Secret             string `json:"secret"`
	Issuer             string `json:"issuer,default=auth_service"`
	AccessExpiry       int    `json:"access_expiry,default=7200"`
	RefreshExpiry      int    `json:"refresh_expiry,default=7776000"`
	AccessTokenType    string `json:"access_token_type,default=access"`
	RefreshTokenType   string `json:"refresh_token_type,default=refresh"`
	ExpiringSoonWindow int    `json:"expiring_soon_window,default=900"`
}

type AuthConfig struct {
	StateTTLSecond        int            `json:"state_ttl_second,default=300"`
	PhoneCaptchaTTLSecond int            `json:"phone_captcha_ttl_second,default=300"`
	GuestDeviceTTLSecond  int            `json:"guest_device_ttl_second,default=600"`
	Tenants               []TenantConfig `json:"tenants,optional"`
}

type TenantConfig struct {
	Key                     string           `json:"key"`
	Name                    string           `json:"name"`
	Enabled                 bool             `json:"enabled"`
	DefaultAvatarURL        string           `json:"default_avatar_url,optional"`
	LegacyDefaultAvatarURLs []string         `json:"legacy_default_avatar_urls,optional"`
	BridgeBaseURL           string           `json:"bridge_base_url,optional"`
	BridgeAuthKey           string           `json:"bridge_auth_key,optional"`
	Providers               []ProviderConfig `json:"providers,optional"`
}

type ProviderConfig struct {
	Provider       string             `json:"provider"`
	ClientType     string             `json:"client_type"`
	Enabled        bool               `json:"enabled"`
	AppID          string             `json:"app_id,optional"`
	AppSecret      string             `json:"app_secret,optional"`
	RedirectURI    string             `json:"redirect_uri,optional"`
	Scope          string             `json:"scope,optional"`
	TeamID         string             `json:"team_id,optional"`
	ClientID       string             `json:"client_id,optional"`
	KeyID          string             `json:"key_id,optional"`
	SigningKey     string             `json:"signing_key,optional"`
	TestPhone      string             `json:"test_phone,optional"`
	TestCaptcha    string             `json:"test_captcha,optional"`
	TestCaptchaKey string             `json:"test_captcha_key,optional"`
	ExtraJSON      string             `json:"extra_json,optional"`
	SMS            *ProviderSMSConfig `json:"sms,optional"`
}

type ProviderSMSConfig struct {
	SecretID       string                               `json:"secret_id,optional"`
	SecretKey      string                               `json:"secret_key,optional"`
	AppKey         string                               `json:"app_key,optional"`
	SmsSDKAppID    string                               `json:"sms_sdk_app_id,optional"`
	SignName       string                               `json:"sign_name,optional"`
	TemplateID     string                               `json:"template_id,optional"`
	TemplateParams []string                             `json:"template_params,optional"`
	Templates      map[string]ProviderSMSTemplateConfig `json:"templates,optional"`
	Region         string                               `json:"region,optional"`
}

type ProviderSMSTemplateConfig struct {
	TemplateID string   `json:"template_id,optional"`
	Params     []string `json:"params,optional"`
}

func (p ProviderConfig) EffectiveExtraJSON() string {
	raw := strings.TrimSpace(p.ExtraJSON)
	if p.SMS == nil {
		return raw
	}

	extra := map[string]any{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &extra)
	}
	if extra == nil {
		extra = map[string]any{}
	}

	sms := map[string]any{}
	if existing, ok := extra["sms"].(map[string]any); ok {
		for key, value := range existing {
			sms[key] = value
		}
	}
	mergeProviderSMSConfig(sms, p.SMS)
	extra["sms"] = sms

	payload, err := json.Marshal(extra)
	if err != nil {
		return raw
	}
	return string(payload)
}

func mergeProviderSMSConfig(target map[string]any, sms *ProviderSMSConfig) {
	if sms == nil {
		return
	}
	setNonEmpty(target, "secret_id", sms.SecretID)
	setNonEmpty(target, "secret_key", sms.SecretKey)
	setNonEmpty(target, "app_key", sms.AppKey)
	setNonEmpty(target, "sms_sdk_app_id", sms.SmsSDKAppID)
	setNonEmpty(target, "sign_name", sms.SignName)
	setNonEmpty(target, "template_id", sms.TemplateID)
	if len(sms.TemplateParams) > 0 {
		target["template_params"] = sms.TemplateParams
	}
	setNonEmpty(target, "region", sms.Region)

	if len(sms.Templates) == 0 {
		return
	}
	templates := make(map[string]any, len(sms.Templates))
	for scene, template := range sms.Templates {
		templateMap := map[string]any{}
		setNonEmpty(templateMap, "template_id", template.TemplateID)
		if len(template.Params) > 0 {
			templateMap["params"] = template.Params
		}
		templates[normalizePhoneCaptchaScene(scene)] = templateMap
	}
	target["templates"] = templates
}

func setNonEmpty(target map[string]any, key string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	target[key] = trimmed
}

func (c *Config) Validate() error {
	normalizeConfig(c)

	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be greater than 0")
	}
	if strings.TrimSpace(c.MySQL.Host) == "" || c.MySQL.Port <= 0 || strings.TrimSpace(c.MySQL.User) == "" || strings.TrimSpace(c.MySQL.Database) == "" {
		return fmt.Errorf("mysql config is incomplete")
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		return fmt.Errorf("redis.addr is required")
	}
	if strings.TrimSpace(c.JWT.Secret) == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	if err := validateTenantConfigs(c.Auth.Tenants); err != nil {
		return err
	}

	return nil
}

func validateTenantConfigs(tenants []TenantConfig) error {
	seen := make(map[string]struct{}, len(tenants))

	for idx, tenant := range tenants {
		tenantKey := normalizeKey(tenant.Key)
		if tenantKey == "" {
			return fmt.Errorf("auth.tenants[%d].key is required", idx)
		}
		if _, exists := seen[tenantKey]; exists {
			return fmt.Errorf("duplicate auth tenant key: %s", tenant.Key)
		}
		seen[tenantKey] = struct{}{}

		if err := validateTenantProviderConfigs(tenant); err != nil {
			return err
		}
		if err := validateTenantBridgeBaseURL(tenant); err != nil {
			return err
		}
		if err := validateTenantAvatarURLs(tenant); err != nil {
			return err
		}
	}

	return nil
}

func validateTenantProviderConfigs(tenant TenantConfig) error {
	for _, provider := range tenant.Providers {
		normalizedProvider := providerkeys.NormalizeProvider(provider.Provider)
		if normalizedProvider == providerkeys.ProviderFirebaseAuth {
			if providerkeys.NormalizeClientType(provider.ClientType) == "" {
				return fmt.Errorf("auth provider %s requires client_type", provider.Provider)
			}
			if strings.TrimSpace(provider.ClientID) == "" && firebaseProjectIDFromExtraJSON(provider.ExtraJSON) == "" {
				return fmt.Errorf("auth provider %s requires client_id or extra_json.project_id", provider.Provider)
			}
			continue
		}

		if !providerkeys.IsWeChatProvider(normalizedProvider) {
			continue
		}

		expectedClientType := providerkeys.WeChatClientType(normalizedProvider)
		if providerkeys.NormalizeClientType(provider.ClientType) != expectedClientType {
			return fmt.Errorf("auth provider %s must use client_type %s", provider.Provider, expectedClientType)
		}
		if normalizedProvider == providerkeys.ProviderWeChatWeb && strings.TrimSpace(provider.RedirectURI) == "" {
			return fmt.Errorf("auth provider %s requires redirect_uri", provider.Provider)
		}
	}

	return nil
}

func firebaseProjectIDFromExtraJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var cfg struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.ProjectID)
}

func validateTenantBridgeBaseURL(tenant TenantConfig) error {
	bridgeBaseURL := strings.TrimSpace(tenant.BridgeBaseURL)
	if bridgeBaseURL == "" {
		return nil
	}

	parsedURL, err := url.Parse(bridgeBaseURL)
	if err != nil || strings.TrimSpace(parsedURL.Scheme) == "" || strings.TrimSpace(parsedURL.Host) == "" {
		return fmt.Errorf("auth.tenants[%s].bridge_base_url is invalid: %s", tenant.Key, bridgeBaseURL)
	}

	switch strings.ToLower(strings.TrimSpace(parsedURL.Scheme)) {
	case "http", "https":
	default:
		return fmt.Errorf("auth.tenants[%s].bridge_base_url must use http or https: %s", tenant.Key, bridgeBaseURL)
	}

	host := strings.TrimSpace(parsedURL.Hostname())
	switch strings.ToLower(host) {
	case "", "localhost", "host.docker.internal":
		return fmt.Errorf("auth.tenants[%s].bridge_base_url must use container-network hostname or domain, not host loopback/ip alias: %s", tenant.Key, bridgeBaseURL)
	}
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Errorf("auth.tenants[%s].bridge_base_url must use container-network hostname or domain, not host loopback/ip alias: %s", tenant.Key, bridgeBaseURL)
	}

	return nil
}

func validateTenantAvatarURLs(tenant TenantConfig) error {
	if err := validateOptionalHTTPURL(tenant.DefaultAvatarURL); err != nil {
		return fmt.Errorf("auth.tenants[%s].default_avatar_url is invalid: %s", tenant.Key, tenant.DefaultAvatarURL)
	}
	for _, avatarURL := range tenant.LegacyDefaultAvatarURLs {
		if err := validateOptionalHTTPURL(avatarURL); err != nil {
			return fmt.Errorf("auth.tenants[%s].legacy_default_avatar_urls contains invalid url: %s", tenant.Key, avatarURL)
		}
	}
	return nil
}

func validateOptionalHTTPURL(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsedURL, err := url.Parse(trimmed)
	if err != nil || strings.TrimSpace(parsedURL.Scheme) == "" || strings.TrimSpace(parsedURL.Host) == "" {
		return fmt.Errorf("invalid url")
	}
	switch strings.ToLower(strings.TrimSpace(parsedURL.Scheme)) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("unsupported scheme")
	}
}

func (c ServerConfig) RestConf(logConf logx.LogConf) rest.RestConf {
	timeout := c.RequestTimeoutMS
	if timeout <= 0 {
		timeout = int64(maxInt(c.ReadTimeout, c.WriteTimeout)) * int64(time.Second/time.Millisecond)
	}

	return rest.RestConf{
		ServiceConf: service.ServiceConf{
			Name: c.Name,
			Log:  logConf,
			Mode: c.Mode,
		},
		Host:             c.Host,
		Port:             c.Port,
		Timeout:          timeout,
		MaxConns:         c.MaxConns,
		MaxBytes:         c.MaxBytes,
		Middlewares:      c.Middlewares,
		TraceIgnorePaths: c.TraceIgnorePaths,
	}
}

func (c MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Database,
	)
}

func (c LogConfig) LogConf(serviceName string) logx.LogConf {
	path := resolveLogDir(c.File)
	mode := "console"
	if path != "" {
		mode = "file"
	}

	return logx.LogConf{
		ServiceName: serviceName,
		Mode:        mode,
		Encoding:    fallbackString(c.Encoding, "json"),
		Path:        path,
		Level:       fallbackString(c.Level, "info"),
		Stat:        true,
	}
}

func (c JWTConfig) SessionConfig() session.Config {
	return session.Config{
		SecretKey:          c.Secret,
		Issuer:             c.Issuer,
		AccessExpiry:       time.Duration(c.AccessExpiry) * time.Second,
		RefreshExpiry:      time.Duration(c.RefreshExpiry) * time.Second,
		AccessTokenType:    c.AccessTokenType,
		RefreshTokenType:   c.RefreshTokenType,
		ExpiringSoonWindow: time.Duration(c.ExpiringSoonWindow) * time.Second,
	}
}

func normalizeConfig(cfg *Config) {
	cfg.Server.Name = fallbackString(cfg.Server.Name, "auth_service")
	cfg.Server.Mode = fallbackString(cfg.Server.Mode, service.ProMode)
	cfg.Server.Host = fallbackString(cfg.Server.Host, "0.0.0.0")
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout <= 0 {
		cfg.Server.ReadTimeout = 30
	}
	if cfg.Server.WriteTimeout <= 0 {
		cfg.Server.WriteTimeout = 30
	}
	if cfg.Server.MaxConns <= 0 {
		cfg.Server.MaxConns = 10000
	}
	if cfg.Server.MaxBytes <= 0 {
		cfg.Server.MaxBytes = 1048576
	}
	if len(cfg.Server.AllowOrigins) == 0 {
		cfg.Server.AllowOrigins = []string{"*"}
	}
	if len(cfg.Server.AllowMethods) == 0 {
		cfg.Server.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if len(cfg.Server.AllowHeaders) == 0 {
		cfg.Server.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"}
	}

	cfg.MySQL.Host = fallbackString(cfg.MySQL.Host, "127.0.0.1")
	if cfg.MySQL.Port <= 0 {
		cfg.MySQL.Port = 3306
	}
	cfg.MySQL.User = fallbackString(cfg.MySQL.User, "root")
	cfg.MySQL.Database = fallbackString(cfg.MySQL.Database, "auth_service")
	if cfg.MySQL.MaxOpenConns <= 0 {
		cfg.MySQL.MaxOpenConns = 100
	}
	if cfg.MySQL.MaxIdleConns <= 0 {
		cfg.MySQL.MaxIdleConns = 10
	}
	if cfg.MySQL.ConnMaxLifetime <= 0 {
		cfg.MySQL.ConnMaxLifetime = 3600
	}
	if cfg.MySQL.ConnMaxIdleTime <= 0 {
		cfg.MySQL.ConnMaxIdleTime = cfg.MySQL.ConnMaxLifetime
	}

	cfg.Redis.Addr = fallbackString(cfg.Redis.Addr, "127.0.0.1:6379")
	if cfg.Redis.PoolSize <= 0 {
		cfg.Redis.PoolSize = 10
	}
	if cfg.Redis.MinIdleConns <= 0 {
		cfg.Redis.MinIdleConns = maxInt(cfg.Redis.PoolSize/2, 1)
	}
	if cfg.Redis.DialTimeoutMS <= 0 {
		cfg.Redis.DialTimeoutMS = 5000
	}
	if cfg.Redis.ReadTimeoutMS <= 0 {
		cfg.Redis.ReadTimeoutMS = 3000
	}
	if cfg.Redis.WriteTimeoutMS <= 0 {
		cfg.Redis.WriteTimeoutMS = 3000
	}
	if cfg.Redis.PoolTimeoutMS <= 0 {
		cfg.Redis.PoolTimeoutMS = 4000
	}

	cfg.Log.Level = fallbackString(cfg.Log.Level, "info")
	cfg.Log.Encoding = fallbackString(cfg.Log.Encoding, "json")
	cfg.Log.File = fallbackString(cfg.Log.File, "logs/app.log")

	cfg.JWT.Secret = strings.TrimSpace(cfg.JWT.Secret)
	cfg.JWT.Issuer = fallbackString(cfg.JWT.Issuer, "auth_service")
	if cfg.JWT.AccessExpiry <= 0 {
		cfg.JWT.AccessExpiry = 7200
	}
	if cfg.JWT.RefreshExpiry <= 0 {
		cfg.JWT.RefreshExpiry = 7776000
	}
	cfg.JWT.AccessTokenType = fallbackString(cfg.JWT.AccessTokenType, "access")
	cfg.JWT.RefreshTokenType = fallbackString(cfg.JWT.RefreshTokenType, "refresh")
	if cfg.JWT.ExpiringSoonWindow <= 0 {
		cfg.JWT.ExpiringSoonWindow = 900
	}

	if cfg.Auth.StateTTLSecond <= 0 {
		cfg.Auth.StateTTLSecond = 300
	}
	if cfg.Auth.PhoneCaptchaTTLSecond <= 0 {
		cfg.Auth.PhoneCaptchaTTLSecond = 300
	}
	if cfg.Auth.GuestDeviceTTLSecond <= 0 {
		cfg.Auth.GuestDeviceTTLSecond = 600
	}

	for tenantIndex := range cfg.Auth.Tenants {
		tenant := &cfg.Auth.Tenants[tenantIndex]
		tenant.Key = normalizeKey(tenant.Key)
		tenant.Name = strings.TrimSpace(tenant.Name)
		tenant.DefaultAvatarURL = strings.TrimSpace(tenant.DefaultAvatarURL)
		for avatarIndex := range tenant.LegacyDefaultAvatarURLs {
			tenant.LegacyDefaultAvatarURLs[avatarIndex] = strings.TrimSpace(tenant.LegacyDefaultAvatarURLs[avatarIndex])
		}
		tenant.BridgeBaseURL = strings.TrimSpace(tenant.BridgeBaseURL)
		tenant.BridgeAuthKey = strings.TrimSpace(tenant.BridgeAuthKey)
		for providerIndex := range tenant.Providers {
			provider := &tenant.Providers[providerIndex]
			provider.Provider = normalizeKey(provider.Provider)
			provider.ClientType = normalizeKey(provider.ClientType)
			provider.AppID = strings.TrimSpace(provider.AppID)
			provider.AppSecret = strings.TrimSpace(provider.AppSecret)
			provider.RedirectURI = strings.TrimSpace(provider.RedirectURI)
			provider.Scope = strings.TrimSpace(provider.Scope)
			provider.TeamID = strings.TrimSpace(provider.TeamID)
			provider.ClientID = strings.TrimSpace(provider.ClientID)
			provider.KeyID = strings.TrimSpace(provider.KeyID)
			provider.SigningKey = strings.TrimSpace(provider.SigningKey)
			provider.TestPhone = strings.TrimSpace(provider.TestPhone)
			provider.TestCaptcha = strings.TrimSpace(provider.TestCaptcha)
			provider.TestCaptchaKey = strings.TrimSpace(provider.TestCaptchaKey)
			provider.ExtraJSON = strings.TrimSpace(provider.ExtraJSON)
			normalizeProviderSMSConfig(provider.SMS)
		}
	}
}

func normalizeProviderSMSConfig(sms *ProviderSMSConfig) {
	if sms == nil {
		return
	}
	sms.SecretID = strings.TrimSpace(sms.SecretID)
	sms.SecretKey = strings.TrimSpace(sms.SecretKey)
	sms.AppKey = strings.TrimSpace(sms.AppKey)
	sms.SmsSDKAppID = strings.TrimSpace(sms.SmsSDKAppID)
	sms.SignName = strings.TrimSpace(sms.SignName)
	sms.TemplateID = strings.TrimSpace(sms.TemplateID)
	sms.TemplateParams = normalizeOptionalStringSlice(sms.TemplateParams)
	sms.Region = strings.TrimSpace(sms.Region)

	if len(sms.Templates) == 0 {
		return
	}
	normalizedTemplates := make(map[string]ProviderSMSTemplateConfig, len(sms.Templates))
	for scene, template := range sms.Templates {
		normalizedTemplates[normalizePhoneCaptchaScene(scene)] = ProviderSMSTemplateConfig{
			TemplateID: strings.TrimSpace(template.TemplateID),
			Params:     normalizeOptionalStringSlice(template.Params),
		}
	}
	sms.Templates = normalizedTemplates
}

func normalizeOptionalStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizePhoneCaptchaScene(scene string) string {
	switch strings.ToLower(strings.TrimSpace(scene)) {
	case "bind":
		return "bind"
	case "rebind":
		return "rebind"
	default:
		return "login"
	}
}

func fallbackString(value string, defaultValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func resolveLogDir(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "logs"
	}

	cleaned := filepath.Clean(trimmed)
	if ext := filepath.Ext(cleaned); ext != "" {
		dir := filepath.Dir(cleaned)
		if dir == "." {
			return "logs"
		}
		return dir
	}

	return cleaned
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
