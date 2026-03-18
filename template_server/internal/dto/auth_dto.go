package dto

import "time"

type ProviderCallbackRequest struct {
	TenantKey         string `json:"tenant_key"`
	ClientType        string `json:"client_type"`
	Code              string `json:"code"`
	State             string `json:"state"`
	AuthorizationCode string `json:"authorization_code"`
	Username          string `json:"username"`
	Email             string `json:"email"`
	Password          string `json:"password"`
	Phone             string `json:"phone"`
	Captcha           string `json:"captcha"`
	CaptchaKey        string `json:"captcha_key"`
	DeviceID          string `json:"device_id"`
	DisplayName       string `json:"display_name"`
	AvatarURL         string `json:"avatar_url"`
	CurrentUserID     uint   `json:"current_user_id"`
	CurrentUserRole   string `json:"current_user_role"`
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
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
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

type AuthUserResponse struct {
	ID          uint       `json:"id"`
	TenantKey   string     `json:"tenant_key"`
	DisplayName string     `json:"display_name"`
	AvatarURL   string     `json:"avatar_url"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type SessionResponse struct {
	TenantKey    string           `json:"tenant_key"`
	Provider     string           `json:"provider"`
	ClientType   string           `json:"client_type"`
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int64            `json:"expires_in"`
	User         AuthUserResponse `json:"user"`
}
