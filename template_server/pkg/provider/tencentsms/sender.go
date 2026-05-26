package tencentsms

import (
	"fmt"
	"strconv"
	"strings"

	authphone "github.com/darren-you/auth_service/template_server/pkg/phone"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const defaultRegion = "ap-guangzhou"

type Config struct {
	SecretID       string
	SecretKey      string
	AppKey         string
	SmsSDKAppID    string
	SignName       string
	TemplateID     string
	TemplateParams []string
	Templates      map[string]TemplateConfig
	Region         string
}

type TemplateConfig struct {
	TemplateID string
	Params     []string
}

type Sender struct {
	config Config
}

func NewSender(cfg Config) *Sender {
	return &Sender{config: cfg}
}

func (s *Sender) SendCaptcha(message authphone.CaptchaMessage) error {
	credential := common.NewCredential(strings.TrimSpace(s.config.SecretID), strings.TrimSpace(s.config.SecretKey))
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.ReqMethod = "POST"
	cpf.HttpProfile.ReqTimeout = 30
	cpf.Language = "zh-CN"

	region := strings.TrimSpace(s.config.Region)
	if region == "" {
		region = defaultRegion
	}
	client, err := tencentsms.NewClient(credential, region, cpf)
	if err != nil {
		return err
	}

	templateID, params := s.resolveTemplate(message.Scene)
	if strings.TrimSpace(templateID) == "" {
		return fmt.Errorf("tencent sms template id is required for scene %s", authphone.NormalizeCaptchaScene(message.Scene))
	}
	request := tencentsms.NewSendSmsRequest()
	request.SmsSdkAppId = common.StringPtr(strings.TrimSpace(s.config.SmsSDKAppID))
	request.SignName = common.StringPtr(strings.TrimSpace(s.config.SignName))
	request.TemplateId = common.StringPtr(templateID)
	request.PhoneNumberSet = common.StringPtrs([]string{strings.TrimSpace(message.Phone)})
	request.TemplateParamSet = common.StringPtrs(resolveTemplateParamValues(params, message))

	response, err := client.SendSms(request)
	if err != nil {
		return err
	}
	for _, status := range response.Response.SendStatusSet {
		if status == nil || status.Code == nil || *status.Code == "Ok" {
			continue
		}
		phoneNumber := ""
		if status.PhoneNumber != nil {
			phoneNumber = *status.PhoneNumber
		}
		message := ""
		if status.Message != nil {
			message = *status.Message
		}
		return fmt.Errorf("failed to send sms to %s: %s - %s", phoneNumber, *status.Code, message)
	}
	return nil
}

func (s *Sender) resolveTemplate(scene string) (string, []string) {
	normalizedScene := authphone.NormalizeCaptchaScene(scene)
	if s.config.Templates != nil {
		if template, ok := s.config.Templates[normalizedScene]; ok {
			return strings.TrimSpace(template.TemplateID), normalizeTemplateParams(template.Params)
		}
		if normalizedScene == "rebind" {
			if template, ok := s.config.Templates["bind"]; ok {
				return strings.TrimSpace(template.TemplateID), normalizeTemplateParams(template.Params)
			}
		}
	}

	return strings.TrimSpace(s.config.TemplateID), normalizeTemplateParams(s.config.TemplateParams)
}

func normalizeTemplateParams(params []string) []string {
	if len(params) == 0 {
		return []string{"captcha", "expire_minutes"}
	}
	result := make([]string, 0, len(params))
	for _, param := range params {
		normalized := strings.ToLower(strings.TrimSpace(param))
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return []string{"captcha", "expire_minutes"}
	}
	return result
}

func resolveTemplateParamValues(params []string, message authphone.CaptchaMessage) []string {
	values := make([]string, 0, len(params))
	for _, param := range normalizeTemplateParams(params) {
		switch param {
		case "captcha":
			values = append(values, strings.TrimSpace(message.Captcha))
		case "expire_minutes":
			values = append(values, strconv.Itoa(message.ExpireMinutes))
		}
	}
	return values
}
