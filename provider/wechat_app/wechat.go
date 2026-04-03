package wechatapp

import internal "github.com/darren-you/auth_service/template_server/pkg/provider/wechat_app"

type Config = internal.Config
type Client = internal.Client
type OAuthToken = internal.OAuthToken
type UserInfo = internal.UserInfo
type AuthCheckResponse = internal.AuthCheckResponse
type APIError = internal.APIError

var NewClient = internal.NewClient
var IsRetryableTokenError = internal.IsRetryableTokenError
