package session

import (
	stdErrors "errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	DefaultAccessTokenType  = "access"
	DefaultRefreshTokenType = "refresh"
)

var (
	ErrTokenNotProvided = stdErrors.New("token not provided")
	ErrInvalidToken     = stdErrors.New("invalid token")
	ErrExpiredToken     = stdErrors.New("token expired")
)

type Config struct {
	SecretKey          string
	Issuer             string
	AccessExpiry       time.Duration
	RefreshExpiry      time.Duration
	AccessTokenType    string
	RefreshTokenType   string
	ExpiringSoonWindow time.Duration
}

type Claims struct {
	AuthUserID uint   `json:"auth_user_id,omitempty"`
	UserID     uint   `json:"user_id"`
	Username   string `json:"username,omitempty"`
	Email      string `json:"email,omitempty"`
	Role       string `json:"role,omitempty"`
	TenantKey  string `json:"tenant_key,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	Status     string `json:"status,omitempty"`
	TokenType  string `json:"token_type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

func (c Config) normalizedIssuer() string {
	if strings.TrimSpace(c.Issuer) != "" {
		return strings.TrimSpace(c.Issuer)
	}
	return "auth_service"
}

func (c Config) normalizedAccessTokenType() string {
	if strings.TrimSpace(c.AccessTokenType) != "" {
		return strings.TrimSpace(c.AccessTokenType)
	}
	return DefaultAccessTokenType
}

func (c Config) normalizedRefreshTokenType() string {
	if strings.TrimSpace(c.RefreshTokenType) != "" {
		return strings.TrimSpace(c.RefreshTokenType)
	}
	return DefaultRefreshTokenType
}

func (c Config) NormalizedExpiringSoonWindow() time.Duration {
	if c.ExpiringSoonWindow > 0 {
		return c.ExpiringSoonWindow
	}
	return 15 * time.Minute
}

func (c Config) validate() error {
	if strings.TrimSpace(c.SecretKey) == "" {
		return fmt.Errorf("secret key is required")
	}
	if c.AccessExpiry <= 0 {
		return fmt.Errorf("access expiry must be greater than zero")
	}
	if c.RefreshExpiry <= 0 {
		return fmt.Errorf("refresh expiry must be greater than zero")
	}
	return nil
}

func GenerateAccessToken(userID uint, username, email, role string, cfg Config) (string, error) {
	return generateAccessTokenWithProfile(0, userID, username, email, role, "", "", "", cfg)
}

func GenerateAccessTokenWithProfile(userID uint, username, email, role, tenantKey, avatarURL, status string, cfg Config) (string, error) {
	return generateAccessTokenWithProfile(0, userID, username, email, role, tenantKey, avatarURL, status, cfg)
}

func GenerateAccessTokenWithAuthUserProfile(
	authUserID uint,
	userID uint,
	username, email, role, tenantKey, avatarURL, status string,
	cfg Config,
) (string, error) {
	return generateAccessTokenWithProfile(authUserID, userID, username, email, role, tenantKey, avatarURL, status, cfg)
}

func generateAccessTokenWithProfile(authUserID uint, userID uint, username, email, role, tenantKey, avatarURL, status string, cfg Config) (string, error) {
	if err := cfg.validate(); err != nil {
		return "", err
	}

	now := time.Now()
	claims := Claims{
		AuthUserID: authUserID,
		UserID:     userID,
		Username:   strings.TrimSpace(username),
		Email:      strings.TrimSpace(email),
		Role:       strings.TrimSpace(role),
		TenantKey:  strings.TrimSpace(tenantKey),
		AvatarURL:  strings.TrimSpace(avatarURL),
		Status:     strings.TrimSpace(status),
		TokenType:  cfg.normalizedAccessTokenType(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.AccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.normalizedIssuer(),
			Subject:   fmt.Sprintf("%d", userID),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.SecretKey))
}

func GenerateRefreshToken(userID uint, cfg Config) (string, error) {
	if err := cfg.validate(); err != nil {
		return "", err
	}

	now := time.Now()
	claims := Claims{
		UserID:    userID,
		TokenType: cfg.normalizedRefreshTokenType(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.RefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    cfg.normalizedIssuer(),
			Subject:   fmt.Sprintf("%d", userID),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.SecretKey))
}

func GenerateTokenPair(userID uint, username, email, role string, cfg Config) (*TokenPair, error) {
	return GenerateTokenPairWithProfile(userID, username, email, role, "", "", "", cfg)
}

func GenerateTokenPairWithProfile(userID uint, username, email, role, tenantKey, avatarURL, status string, cfg Config) (*TokenPair, error) {
	return GenerateTokenPairWithAuthUserProfile(0, userID, username, email, role, tenantKey, avatarURL, status, cfg)
}

func GenerateTokenPairWithAuthUserProfile(
	authUserID uint,
	userID uint,
	username, email, role, tenantKey, avatarURL, status string,
	cfg Config,
) (*TokenPair, error) {
	accessToken, err := GenerateAccessTokenWithAuthUserProfile(authUserID, userID, username, email, role, tenantKey, avatarURL, status, cfg)
	if err != nil {
		return nil, err
	}
	refreshToken, err := GenerateRefreshToken(userID, cfg)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(cfg.AccessExpiry / time.Second),
	}, nil
}

func ParseToken(tokenString string, cfg Config) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.SecretKey), nil
	})
	if err != nil {
		if stdErrors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	normalizeClaims(claims)
	if claims.UserID == 0 {
		if parsedUserID, ok := parseSubjectUserID(claims.Subject); ok {
			claims.UserID = parsedUserID
		}
	}
	if claims.UserID == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func ParseRefreshTokenIgnoringExpiry(tokenString string, cfg Config) (*Claims, error) {
	parser := jwt.NewParser(
		jwt.WithLeeway(365*24*time.Hour),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	token, err := parser.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.SecretKey), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	normalizeClaims(claims)
	if claims.UserID == 0 {
		if parsedUserID, ok := parseSubjectUserID(claims.Subject); ok {
			claims.UserID = parsedUserID
		}
	}
	if claims.UserID == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func ParseAccessToken(tokenString string, cfg Config) (*Claims, error) {
	claims, err := ParseToken(tokenString, cfg)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != cfg.normalizedAccessTokenType() {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func ParseRefreshToken(tokenString string, cfg Config) (*Claims, error) {
	claims, err := ParseToken(tokenString, cfg)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != cfg.normalizedRefreshTokenType() {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func ExtractBearerToken(authHeader string) (string, error) {
	trimmed := strings.TrimSpace(authHeader)
	if trimmed == "" {
		return "", ErrTokenNotProvided
	}
	if len(trimmed) < len("Bearer ")+1 {
		return "", ErrInvalidToken
	}
	if !strings.HasPrefix(trimmed, "Bearer ") {
		return "", ErrInvalidToken
	}
	token := strings.TrimSpace(strings.TrimPrefix(trimmed, "Bearer "))
	if token == "" {
		return "", ErrInvalidToken
	}
	return token, nil
}

func IsAccessTokenExpiringSoon(claims *Claims, cfg Config, now time.Time) bool {
	if claims == nil || claims.ExpiresAt == nil {
		return false
	}
	window := cfg.NormalizedExpiringSoonWindow()
	if window <= 0 {
		return false
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if !now.IsZero() {
		remaining = claims.ExpiresAt.Time.Sub(now)
	}
	return remaining > 0 && remaining < window
}

func normalizeClaims(claims *Claims) {
	if claims == nil {
		return
	}
	claims.Username = strings.TrimSpace(claims.Username)
	claims.Email = strings.TrimSpace(claims.Email)
	claims.Role = strings.TrimSpace(claims.Role)
	claims.TokenType = strings.TrimSpace(claims.TokenType)
	claims.Subject = strings.TrimSpace(claims.Subject)
}

func parseSubjectUserID(subject string) (uint, bool) {
	trimmed := strings.TrimSpace(subject)
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return uint(value), true
}
