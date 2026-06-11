// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/darren-you/auth_service/providerkeys"
	"github.com/darren-you/auth_service/template_server/internal/config"
	appErrors "github.com/darren-you/auth_service/template_server/internal/errorx"
	"github.com/darren-you/auth_service/template_server/internal/svc"
	"github.com/darren-you/auth_service/template_server/internal/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	webGateTokenType         = "web_gate"
	webGateDefaultCookiePath = "/"
	webGateDefaultTTL        = 30 * 24 * time.Hour
)

type webGateProviderOptions struct {
	CookieName        string `json:"cookie_name"`
	CookiePath        string `json:"cookie_path"`
	CookieSameSite    string `json:"cookie_same_site"`
	CookieSecure      *bool  `json:"cookie_secure"`
	CookiePartitioned bool   `json:"cookie_partitioned"`
	SessionTTLSecond  int    `json:"session_ttl_second"`
}

type webGateClaims struct {
	TenantKey  string `json:"tenant_key"`
	ClientType string `json:"client_type"`
	TokenType  string `json:"token_type"`
	jwt.RegisteredClaims
}

type WebGateLoginResult struct {
	Cookie    *http.Cookie
	ExpiresIn int64
}

type WebGateLogoutResult struct {
	Cookie *http.Cookie
}

type WebGateLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWebGateLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WebGateLoginLogic {
	return &WebGateLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WebGateLoginLogic) WebGateLogin(req *types.WebGateLoginReq) (*WebGateLoginResult, error) {
	tenantKey := normalize(req.TenantKey)
	clientType := normalize(req.ClientType)
	password := strings.TrimSpace(req.Password)
	if tenantKey == "" || clientType == "" || password == "" {
		return nil, appErrors.ErrBadRequest
	}

	providerConfig, err := resolveWebGateProvider(l.svcCtx.Config, tenantKey, clientType)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(providerConfig.AppSecret)) != 1 {
		return nil, appErrors.ErrForbidden
	}

	options, err := resolveWebGateOptions(tenantKey, providerConfig)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := signWebGateToken(l.svcCtx.Config, tenantKey, clientType, options.ttl)
	if err != nil {
		return nil, appErrors.New(appErrors.ErrInternalServer.Code, appErrors.ErrInternalServer.HTTPStatus, appErrors.ErrInternalServer.Message, err)
	}

	return &WebGateLoginResult{
		Cookie:    buildWebGateCookie(options, token, expiresAt),
		ExpiresIn: int64(options.ttl / time.Second),
	}, nil
}

type WebGateVerifyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWebGateVerifyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WebGateVerifyLogic {
	return &WebGateVerifyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WebGateVerifyLogic) WebGateVerify(req *types.WebGateVerifyReq, cookies []*http.Cookie) error {
	tenantKey := normalize(req.TenantKey)
	clientType := normalize(req.ClientType)
	if tenantKey == "" || clientType == "" {
		return appErrors.ErrUnauthorized
	}

	providerConfig, err := resolveWebGateProvider(l.svcCtx.Config, tenantKey, clientType)
	if err != nil {
		return appErrors.ErrUnauthorized
	}
	options, err := resolveWebGateOptions(tenantKey, providerConfig)
	if err != nil {
		return appErrors.ErrUnauthorized
	}

	token := webGateCookieValue(cookies, options.cookieName)
	if token == "" {
		return appErrors.ErrUnauthorized
	}
	claims, err := parseWebGateToken(l.svcCtx.Config, token)
	if err != nil {
		return appErrors.ErrUnauthorized
	}
	if normalize(claims.TenantKey) != tenantKey || normalize(claims.ClientType) != clientType {
		return appErrors.ErrUnauthorized
	}

	return nil
}

type WebGateLogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWebGateLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WebGateLogoutLogic {
	return &WebGateLogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WebGateLogoutLogic) WebGateLogout(req *types.WebGateLogoutReq) (*WebGateLogoutResult, error) {
	tenantKey := normalize(req.TenantKey)
	clientType := normalize(req.ClientType)
	if tenantKey == "" || clientType == "" {
		return nil, appErrors.ErrBadRequest
	}

	providerConfig, err := resolveWebGateProvider(l.svcCtx.Config, tenantKey, clientType)
	if err != nil {
		return nil, err
	}
	options, err := resolveWebGateOptions(tenantKey, providerConfig)
	if err != nil {
		return nil, err
	}

	return &WebGateLogoutResult{
		Cookie: clearWebGateCookie(options),
	}, nil
}

type resolvedWebGateOptions struct {
	cookieName        string
	cookiePath        string
	cookieSameSite    http.SameSite
	cookieSecure      bool
	cookiePartitioned bool
	ttl               time.Duration
}

func resolveWebGateProvider(cfg config.Config, tenantKey string, clientType string) (config.ProviderConfig, error) {
	for _, tenant := range cfg.Auth.Tenants {
		if normalize(tenant.Key) != tenantKey || !tenant.Enabled {
			continue
		}
		for _, providerConfig := range tenant.Providers {
			if providerkeys.NormalizeProvider(providerConfig.Provider) != providerkeys.ProviderWebGate {
				continue
			}
			if providerkeys.NormalizeClientType(providerConfig.ClientType) != clientType || !providerConfig.Enabled {
				continue
			}
			if strings.TrimSpace(providerConfig.AppSecret) == "" {
				return config.ProviderConfig{}, appErrors.ErrForbidden
			}
			return providerConfig, nil
		}
		return config.ProviderConfig{}, appErrors.ErrForbidden
	}
	return config.ProviderConfig{}, appErrors.ErrForbidden
}

func resolveWebGateOptions(tenantKey string, providerConfig config.ProviderConfig) (resolvedWebGateOptions, error) {
	raw := strings.TrimSpace(providerConfig.ExtraJSON)
	options := webGateProviderOptions{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &options); err != nil {
			return resolvedWebGateOptions{}, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "web gate provider extra_json is invalid", err)
		}
	}

	cookieName := strings.TrimSpace(options.CookieName)
	if cookieName == "" {
		cookieName = tenantKey + "_gate"
	}
	if err := (&http.Cookie{Name: cookieName, Value: "x"}).Valid(); err != nil {
		return resolvedWebGateOptions{}, appErrors.New(appErrors.ErrConfigInvalid.Code, appErrors.ErrConfigInvalid.HTTPStatus, "web gate cookie_name is invalid", err)
	}

	cookiePath := strings.TrimSpace(options.CookiePath)
	if cookiePath == "" {
		cookiePath = webGateDefaultCookiePath
	}

	cookieSecure := true
	if options.CookieSecure != nil {
		cookieSecure = *options.CookieSecure
	}
	if options.CookiePartitioned {
		cookieSecure = true
	}

	ttl := webGateDefaultTTL
	if options.SessionTTLSecond > 0 {
		ttl = time.Duration(options.SessionTTLSecond) * time.Second
	}

	return resolvedWebGateOptions{
		cookieName:        cookieName,
		cookiePath:        cookiePath,
		cookieSameSite:    parseWebGateSameSite(options.CookieSameSite),
		cookieSecure:      cookieSecure,
		cookiePartitioned: options.CookiePartitioned,
		ttl:               ttl,
	}, nil
}

func parseWebGateSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func signWebGateToken(cfg config.Config, tenantKey string, clientType string, ttl time.Duration) (string, time.Time, error) {
	if strings.TrimSpace(cfg.JWT.Secret) == "" {
		return "", time.Time{}, fmt.Errorf("jwt secret is required")
	}
	now := time.Now()
	expiresAt := now.Add(ttl)
	claims := webGateClaims{
		TenantKey:  tenantKey,
		ClientType: clientType,
		TokenType:  webGateTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    webGateIssuer(cfg),
			Subject:   tenantKey + ":" + clientType,
			Audience:  []string{webGateTokenType},
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(cfg.JWT.Secret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signedToken, expiresAt, nil
}

func parseWebGateToken(cfg config.Config, rawToken string) (*webGateClaims, error) {
	tokenString := strings.TrimSpace(rawToken)
	if tokenString == "" {
		return nil, appErrors.ErrUnauthorized
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&webGateClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(cfg.JWT.Secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(webGateIssuer(cfg)),
		jwt.WithAudience(webGateTokenType),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, appErrors.ErrUnauthorized
		}
		return nil, appErrors.ErrUnauthorized
	}
	claims, ok := token.Claims.(*webGateClaims)
	if !ok || !token.Valid {
		return nil, appErrors.ErrUnauthorized
	}
	if normalize(claims.TokenType) != webGateTokenType || normalize(claims.TenantKey) == "" || normalize(claims.ClientType) == "" {
		return nil, appErrors.ErrUnauthorized
	}
	return claims, nil
}

func webGateIssuer(cfg config.Config) string {
	issuer := strings.TrimSpace(cfg.JWT.Issuer)
	if issuer == "" {
		return "auth_service"
	}
	return issuer
}

func buildWebGateCookie(options resolvedWebGateOptions, token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:        options.cookieName,
		Value:       token,
		Path:        options.cookiePath,
		Expires:     expiresAt,
		MaxAge:      int(options.ttl / time.Second),
		Secure:      options.cookieSecure,
		HttpOnly:    true,
		SameSite:    options.cookieSameSite,
		Partitioned: options.cookiePartitioned,
	}
}

func clearWebGateCookie(options resolvedWebGateOptions) *http.Cookie {
	return &http.Cookie{
		Name:        options.cookieName,
		Value:       "",
		Path:        options.cookiePath,
		Expires:     time.Unix(0, 0).UTC(),
		MaxAge:      -1,
		Secure:      options.cookieSecure,
		HttpOnly:    true,
		SameSite:    options.cookieSameSite,
		Partitioned: options.cookiePartitioned,
	}
}

func webGateCookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		if cookie.Name == name {
			return strings.TrimSpace(cookie.Value)
		}
	}
	return ""
}
