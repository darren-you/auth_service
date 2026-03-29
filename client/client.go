package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	requestIDHeader = "X-Request-ID"
	traceIDKey      = "trace_id"
)

type Config struct {
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Error struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return fmt.Sprintf("auth_service request failed: status=%d code=%d", e.HTTPStatus, e.Code)
}

type ProviderCallbackRequest struct {
	TenantKey         string `json:"tenant_key"`
	ClientType        string `json:"client_type"`
	Code              string `json:"code,omitempty"`
	State             string `json:"state,omitempty"`
	AuthorizationCode string `json:"authorization_code,omitempty"`
	Username          string `json:"username,omitempty"`
	Email             string `json:"email,omitempty"`
	Password          string `json:"password,omitempty"`
	Phone             string `json:"phone,omitempty"`
	Captcha           string `json:"captcha,omitempty"`
	CaptchaKey        string `json:"captcha_key,omitempty"`
	Token             string `json:"token,omitempty"`
	Gyuid             string `json:"gyuid,omitempty"`
	DeviceID          string `json:"device_id,omitempty"`
	DisplayName       string `json:"display_name,omitempty"`
	AvatarURL         string `json:"avatar_url,omitempty"`
	CurrentUserID     uint   `json:"current_user_id,omitempty"`
	CurrentUserRole   string `json:"current_user_role,omitempty"`
}

type PhoneCaptchaSendRequest struct {
	TenantKey  string `json:"tenant_key"`
	ClientType string `json:"client_type"`
	Phone      string `json:"phone"`
}

type GuestDeviceIDRequest struct {
	TenantKey  string `json:"tenant_key"`
	ClientType string `json:"client_type"`
}

type PasswordRegisterRequest struct {
	TenantKey   string `json:"tenant_key"`
	ClientType  string `json:"client_type"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LoginURLResponse struct {
	TenantKey  string `json:"tenant_key"`
	Provider   string `json:"provider"`
	ClientType string `json:"client_type"`
	LoginURL   string `json:"login_url"`
	State      string `json:"state"`
}

type PhoneCaptchaSendResponse struct {
	TenantKey  string `json:"tenant_key"`
	ClientType string `json:"client_type"`
	CaptchaKey string `json:"captcha_key"`
	ExpiresIn  int    `json:"expires_in"`
}

type GuestDeviceIDResponse struct {
	TenantKey  string `json:"tenant_key"`
	ClientType string `json:"client_type"`
	DeviceID   string `json:"device_id"`
	ExpiresIn  int    `json:"expires_in"`
}

type UserProfile struct {
	ID          uint   `json:"id"`
	TenantKey   string `json:"tenant_key"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type SessionResponse struct {
	TenantKey    string      `json:"tenant_key"`
	Provider     string      `json:"provider"`
	ClientType   string      `json:"client_type"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	User         UserProfile `json:"user"`
}

type responseEnvelope[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) GetLoginURL(ctx context.Context, provider string, tenantKey string, clientType string) (*LoginURLResponse, error) {
	query := url.Values{}
	query.Set("tenant_key", strings.TrimSpace(tenantKey))
	query.Set("client_type", strings.TrimSpace(clientType))
	endpoint := fmt.Sprintf("%s/api/v1/auth/providers/%s/login-url?%s", c.baseURL, strings.TrimSpace(provider), query.Encode())
	var resp LoginURLResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ProviderCallback(ctx context.Context, provider string, req ProviderCallbackRequest) (*SessionResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/auth/providers/%s/callback", c.baseURL, strings.TrimSpace(provider))
	var resp SessionResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RegisterPassword(ctx context.Context, req PasswordRegisterRequest) (*SessionResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/auth/providers/password/register", c.baseURL)
	var resp SessionResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SendPhoneCaptcha(ctx context.Context, req PhoneCaptchaSendRequest) (*PhoneCaptchaSendResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/auth/providers/phone/send-captcha", c.baseURL)
	var resp PhoneCaptchaSendResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) IssueGuestDeviceID(ctx context.Context, req GuestDeviceIDRequest) (*GuestDeviceIDResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/auth/providers/guest/device-id", c.baseURL)
	var resp GuestDeviceIDResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (*SessionResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/auth/refresh", c.baseURL)
	var resp SessionResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, RefreshTokenRequest{RefreshToken: strings.TrimSpace(refreshToken)}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Logout(ctx context.Context, refreshToken string) error {
	endpoint := fmt.Sprintf("%s/api/v1/auth/logout", c.baseURL)
	return c.doJSON(ctx, http.MethodPost, endpoint, LogoutRequest{RefreshToken: strings.TrimSpace(refreshToken)}, nil)
}

func (c *Client) doJSON(ctx context.Context, method string, endpoint string, reqBody interface{}, out interface{}) error {
	if c.baseURL == "" {
		return fmt.Errorf("auth_service base url is empty")
	}

	var bodyReader *bytes.Reader
	if reqBody == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request failed: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	propagateRequestID(req, ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request auth_service failed: %w", err)
	}
	defer resp.Body.Close()

	if out == nil {
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return &Error{
				HTTPStatus: resp.StatusCode,
				Code:       resp.StatusCode,
				Message:    fmt.Sprintf("auth_service request failed: status=%d", resp.StatusCode),
			}
		}
		return nil
	}

	var envelope responseEnvelope[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode auth_service response failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || envelope.Code != 200 {
		msg := strings.TrimSpace(envelope.Msg)
		if msg == "" {
			msg = fmt.Sprintf("status=%d", resp.StatusCode)
		}
		return &Error{
			HTTPStatus: resp.StatusCode,
			Code:       envelope.Code,
			Message:    msg,
		}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode auth_service payload failed: %w", err)
	}
	return nil
}

func propagateRequestID(req *http.Request, ctx context.Context) {
	if req == nil || ctx == nil {
		return
	}

	requestID := requestIDFromContext(ctx)
	if requestID == "" {
		return
	}

	req.Header.Set(requestIDHeader, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if requestID, ok := ctx.Value(traceIDKey).(string); ok {
		return strings.TrimSpace(requestID)
	}
	if requestID, ok := ctx.Value(requestIDHeader).(string); ok {
		return strings.TrimSpace(requestID)
	}

	return ""
}
