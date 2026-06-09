package firebaseauth

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultJWKSURL               = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"
	defaultRequestTimeout        = 20 * time.Second
	defaultDialTimeout           = 5 * time.Second
	defaultTLSHandshakeTimeout   = 8 * time.Second
	defaultResponseHeaderTimeout = 15 * time.Second
	defaultIdleConnTimeout       = 90 * time.Second
)

type Config struct {
	ProjectID            string
	JWKSURL              string
	RequestTimeoutSecond int
}

type VerifiedIDToken struct {
	UID            string
	Email          string
	EmailVerified  bool
	DisplayName    string
	AvatarURL      string
	SignInProvider string
	RawClaims      map[string]any
}

type Client struct {
	projectID  string
	jwksURL    string
	httpClient *http.Client
}

type firebaseIDTokenClaims struct {
	Email         string         `json:"email,omitempty"`
	EmailVerified bool           `json:"email_verified,omitempty"`
	Name          string         `json:"name,omitempty"`
	Picture       string         `json:"picture,omitempty"`
	Firebase      map[string]any `json:"firebase,omitempty"`
	jwt.RegisteredClaims
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

type cachedKeys struct {
	expiresAt time.Time
	keys      map[string]*rsa.PublicKey
}

var keyCache = struct {
	sync.Mutex
	byURL map[string]cachedKeys
}{
	byURL: make(map[string]cachedKeys),
}

func NewClient(cfg Config) (*Client, error) {
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("firebase project_id is required")
	}

	timeout := time.Duration(cfg.RequestTimeoutSecond) * time.Second
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}

	jwksURL := strings.TrimSpace(cfg.JWKSURL)
	if jwksURL == "" {
		jwksURL = defaultJWKSURL
	}

	return &Client{
		projectID:  projectID,
		jwksURL:    jwksURL,
		httpClient: newJWKSHTTPClient(timeout),
	}, nil
}

func newJWKSHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   minDuration(timeout, defaultDialTimeout),
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", address)
			},
			TLSHandshakeTimeout:   minDuration(timeout, defaultTLSHandshakeTimeout),
			ResponseHeaderTimeout: minDuration(timeout, defaultResponseHeaderTimeout),
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       defaultIdleConnTimeout,
			TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string) (*VerifiedIDToken, error) {
	trimmedToken := strings.TrimSpace(rawIDToken)
	if trimmedToken == "" {
		return nil, fmt.Errorf("firebase id token is required")
	}

	claims := &firebaseIDTokenClaims{}
	issuer := "https://securetoken.google.com/" + c.projectID
	token, err := jwt.ParseWithClaims(
		trimmedToken,
		claims,
		c.keyfunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(c.projectID),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if token == nil || !token.Valid {
		return nil, fmt.Errorf("invalid firebase id token")
	}

	uid := strings.TrimSpace(claims.Subject)
	if uid == "" || len(uid) > 128 {
		return nil, fmt.Errorf("invalid firebase uid")
	}

	rawClaims := make(map[string]any)
	payload, _ := json.Marshal(claims)
	_ = json.Unmarshal(payload, &rawClaims)

	return &VerifiedIDToken{
		UID:            uid,
		Email:          strings.TrimSpace(claims.Email),
		EmailVerified:  claims.EmailVerified,
		DisplayName:    strings.TrimSpace(claims.Name),
		AvatarURL:      strings.TrimSpace(claims.Picture),
		SignInProvider: firebaseSignInProvider(claims.Firebase),
		RawClaims:      rawClaims,
	}, nil
}

func (c *Client) keyfunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		keyID, _ := token.Header["kid"].(string)
		keyID = strings.TrimSpace(keyID)
		if keyID == "" {
			return nil, fmt.Errorf("firebase id token missing kid")
		}

		keys, err := c.loadPublicKeys(ctx)
		if err != nil {
			return nil, err
		}
		key := keys[keyID]
		if key == nil {
			return nil, fmt.Errorf("firebase public key not found for kid %s", keyID)
		}
		return key, nil
	}
}

func (c *Client) loadPublicKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	now := time.Now()
	keyCache.Lock()
	cached := keyCache.byURL[c.jwksURL]
	if len(cached.keys) > 0 && now.Before(cached.expiresAt) {
		keys := cached.keys
		keyCache.Unlock()
		return keys, nil
	}
	keyCache.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("firebase jwks http status: %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		publicKey, err := key.rsaPublicKey()
		if err != nil {
			return nil, err
		}
		keys[key.KeyID] = publicKey
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("firebase jwks contains no keys")
	}

	expiresAt := now.Add(resolveCacheMaxAge(resp.Header.Get("Cache-Control")))
	keyCache.Lock()
	keyCache.byURL[c.jwksURL] = cachedKeys{
		expiresAt: expiresAt,
		keys:      keys,
	}
	keyCache.Unlock()

	return keys, nil
}

func (k jwkKey) rsaPublicKey() (*rsa.PublicKey, error) {
	if strings.TrimSpace(k.KeyType) != "RSA" {
		return nil, fmt.Errorf("unsupported firebase jwk type: %s", k.KeyType)
	}

	modulusBytes, err := base64.RawURLEncoding.DecodeString(k.Modulus)
	if err != nil {
		return nil, fmt.Errorf("decode firebase jwk modulus failed: %w", err)
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(k.Exponent)
	if err != nil {
		return nil, fmt.Errorf("decode firebase jwk exponent failed: %w", err)
	}

	exponent := 0
	for _, b := range exponentBytes {
		exponent = exponent<<8 + int(b)
	}
	if exponent == 0 {
		return nil, fmt.Errorf("firebase jwk exponent is empty")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func resolveCacheMaxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if !strings.HasPrefix(part, "max-age=") {
			continue
		}
		rawSeconds := strings.TrimPrefix(part, "max-age=")
		seconds, err := strconv.Atoi(rawSeconds)
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return time.Hour
}

func firebaseSignInProvider(firebase map[string]any) string {
	value, _ := firebase["sign_in_provider"].(string)
	return strings.TrimSpace(value)
}
