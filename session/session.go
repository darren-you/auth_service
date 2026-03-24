package session

import internal "github.com/darren-you/auth_service/template_server/pkg/session"

type Config = internal.Config
type Claims = internal.Claims
type TokenPair = internal.TokenPair

const DefaultAccessTokenType = internal.DefaultAccessTokenType
const DefaultRefreshTokenType = internal.DefaultRefreshTokenType

var ErrTokenNotProvided = internal.ErrTokenNotProvided
var ErrInvalidToken = internal.ErrInvalidToken
var ErrExpiredToken = internal.ErrExpiredToken

var GenerateAccessToken = internal.GenerateAccessToken
var GenerateAccessTokenWithProfile = internal.GenerateAccessTokenWithProfile
var GenerateRefreshToken = internal.GenerateRefreshToken
var GenerateTokenPair = internal.GenerateTokenPair
var GenerateTokenPairWithProfile = internal.GenerateTokenPairWithProfile
var ParseToken = internal.ParseToken
var ParseAccessToken = internal.ParseAccessToken
var ParseRefreshToken = internal.ParseRefreshToken
var ParseRefreshTokenIgnoringExpiry = internal.ParseRefreshTokenIgnoringExpiry
var ExtractBearerToken = internal.ExtractBearerToken
