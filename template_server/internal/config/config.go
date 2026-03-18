package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `json:"server" yaml:"server"`
	MySQL  MySQLConfig  `json:"mysql" yaml:"mysql"`
	Redis  RedisConfig  `json:"redis" yaml:"redis"`
	Log    LogConfig    `json:"log" yaml:"log"`
	JWT    JWTConfig    `json:"jwt" yaml:"jwt"`
	Auth   AuthConfig   `json:"auth" yaml:"auth"`
}

type ServerConfig struct {
	Port         int      `json:"port" yaml:"port"`
	ReadTimeout  int      `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout int      `json:"write_timeout" yaml:"write_timeout"`
	AllowOrigins []string `json:"allow_origins" yaml:"allow_origins"`
	AllowMethods []string `json:"allow_methods" yaml:"allow_methods"`
	AllowHeaders []string `json:"allow_headers" yaml:"allow_headers"`
}

type MySQLConfig struct {
	Host            string `json:"host" yaml:"host"`
	Port            int    `json:"port" yaml:"port"`
	User            string `json:"user" yaml:"user"`
	Password        string `json:"password" yaml:"password"`
	DBName          string `json:"database" yaml:"database"`
	MaxOpenConns    int    `json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int    `json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime int    `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `json:"addr" yaml:"addr"`
	Password string `json:"password" yaml:"password"`
	DB       int    `json:"db" yaml:"db"`
	PoolSize int    `json:"pool_size" yaml:"pool_size"`
}

type LogConfig struct {
	Level string `json:"level" yaml:"level"`
	Path  string `json:"file" yaml:"file"`
}

type JWTConfig struct {
	Secret           string `json:"secret" yaml:"secret"`
	Issuer           string `json:"issuer" yaml:"issuer"`
	ExpiresIn        int    `json:"access_expiry" yaml:"access_expiry"`
	RefreshExpiresIn int    `json:"refresh_expiry" yaml:"refresh_expiry"`
}

type AuthConfig struct {
	StateTTLSecond        int            `json:"state_ttl_second" yaml:"state_ttl_second"`
	PhoneCaptchaTTLSecond int            `json:"phone_captcha_ttl_second" yaml:"phone_captcha_ttl_second"`
	GuestDeviceTTLSecond  int            `json:"guest_device_ttl_second" yaml:"guest_device_ttl_second"`
	Tenants               []TenantConfig `json:"tenants" yaml:"tenants"`
}

type TenantConfig struct {
	Key           string           `json:"key" yaml:"key"`
	Name          string           `json:"name" yaml:"name"`
	Enabled       bool             `json:"enabled" yaml:"enabled"`
	BridgeBaseURL string           `json:"bridge_base_url" yaml:"bridge_base_url"`
	BridgeAuthKey string           `json:"bridge_auth_key" yaml:"bridge_auth_key"`
	Providers     []ProviderConfig `json:"providers" yaml:"providers"`
}

type ProviderConfig struct {
	Provider       string `json:"provider" yaml:"provider"`
	ClientType     string `json:"client_type" yaml:"client_type"`
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	AppID          string `json:"app_id" yaml:"app_id"`
	AppSecret      string `json:"app_secret" yaml:"app_secret"`
	RedirectURI    string `json:"redirect_uri" yaml:"redirect_uri"`
	Scope          string `json:"scope" yaml:"scope"`
	TeamID         string `json:"team_id" yaml:"team_id"`
	ClientID       string `json:"client_id" yaml:"client_id"`
	KeyID          string `json:"key_id" yaml:"key_id"`
	SigningKey     string `json:"signing_key" yaml:"signing_key"`
	TestPhone      string `json:"test_phone" yaml:"test_phone"`
	TestCaptcha    string `json:"test_captcha" yaml:"test_captcha"`
	TestCaptchaKey string `json:"test_captcha_key" yaml:"test_captcha_key"`
	ExtraJSON      string `json:"extra_json" yaml:"extra_json"`
}

func LoadConfig() (*Config, error) {
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", cfgPath, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", cfgPath, err)
	}

	normalizeConfig(&cfg)
	return &cfg, nil
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 30,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		},
		MySQL: MySQLConfig{
			Host:            "127.0.0.1",
			Port:            3306,
			User:            "root",
			Password:        "",
			DBName:          "auth_service",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: 3600,
		},
		Redis: RedisConfig{
			Addr:     "127.0.0.1:6379",
			Password: "",
			DB:       2,
			PoolSize: 10,
		},
		Log: LogConfig{
			Level: "info",
			Path:  "logs/app.log",
		},
		JWT: JWTConfig{
			Secret:           "replace-with-a-strong-secret",
			Issuer:           "auth_service",
			ExpiresIn:        3600,
			RefreshExpiresIn: 604800,
		},
		Auth: AuthConfig{
			StateTTLSecond:        300,
			PhoneCaptchaTTLSecond: 300,
			GuestDeviceTTLSecond:  600,
		},
	}
}

func resolveConfigPath() (string, error) {
	candidates := []string{
		filepath.Join("config", "config.yaml"),
		filepath.Join("template_server", "config", "config.yaml"),
		"config.yaml",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("config file not found; tried: %s", strings.Join(candidates, ", "))
}

func normalizeConfig(cfg *Config) {
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout <= 0 {
		cfg.Server.ReadTimeout = 30
	}
	if cfg.Server.WriteTimeout <= 0 {
		cfg.Server.WriteTimeout = 30
	}
	if len(cfg.Server.AllowOrigins) == 0 {
		cfg.Server.AllowOrigins = []string{"*"}
	}
	if len(cfg.Server.AllowMethods) == 0 {
		cfg.Server.AllowMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if len(cfg.Server.AllowHeaders) == 0 {
		cfg.Server.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}

	cfg.MySQL.Host = fallbackString(cfg.MySQL.Host, "127.0.0.1")
	if cfg.MySQL.Port <= 0 {
		cfg.MySQL.Port = 3306
	}
	cfg.MySQL.User = fallbackString(cfg.MySQL.User, "root")
	cfg.MySQL.DBName = fallbackString(cfg.MySQL.DBName, "auth_service")
	if cfg.MySQL.MaxOpenConns <= 0 {
		cfg.MySQL.MaxOpenConns = 100
	}
	if cfg.MySQL.MaxIdleConns <= 0 {
		cfg.MySQL.MaxIdleConns = 10
	}
	if cfg.MySQL.ConnMaxLifetime <= 0 {
		cfg.MySQL.ConnMaxLifetime = 3600
	}

	cfg.Redis.Addr = fallbackString(cfg.Redis.Addr, "127.0.0.1:6379")
	if cfg.Redis.PoolSize <= 0 {
		cfg.Redis.PoolSize = 10
	}

	cfg.Log.Level = fallbackString(cfg.Log.Level, "info")
	cfg.Log.Path = fallbackString(cfg.Log.Path, "logs/app.log")

	cfg.JWT.Secret = fallbackString(cfg.JWT.Secret, "replace-with-a-strong-secret")
	cfg.JWT.Issuer = fallbackString(cfg.JWT.Issuer, "auth_service")
	if cfg.JWT.ExpiresIn <= 0 {
		cfg.JWT.ExpiresIn = 3600
	}
	if cfg.JWT.RefreshExpiresIn <= 0 {
		cfg.JWT.RefreshExpiresIn = 604800
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
		}
	}
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func fallbackString(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
