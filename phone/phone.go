package phone

import internal "github.com/darren-you/auth_service/template_server/pkg/phone"

type Store = internal.Store
type Sender = internal.Sender
type Config = internal.Config
type SendResult = internal.SendResult
type VerifyRequest = internal.VerifyRequest
type Service = internal.Service

var ErrInvalidCaptcha = internal.ErrInvalidCaptcha
var NewService = internal.NewService
