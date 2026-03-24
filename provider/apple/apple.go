package apple

import internal "github.com/darren-you/auth_service/template_server/pkg/provider/apple"

type Config = internal.Config
type Client = internal.Client
type ValidationResponse = internal.ValidationResponse

var NewClient = internal.NewClient
var NewClientWithSecretFile = internal.NewClientWithSecretFile
var IsAuthorizationError = internal.IsAuthorizationError
