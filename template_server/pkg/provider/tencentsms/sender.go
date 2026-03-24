package tencentsms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const defaultRegion = "ap-guangzhou"

type Config struct {
	SecretID    string
	SecretKey   string
	SmsSDKAppID string
	SignName    string
	TemplateID  string
	Region      string
}

type Sender struct {
	config Config
}

func NewSender(cfg Config) *Sender {
	return &Sender{config: cfg}
}

func (s *Sender) SendCaptcha(phone string, expireMinutes int, captcha string) error {
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

	request := tencentsms.NewSendSmsRequest()
	request.SmsSdkAppId = common.StringPtr(strings.TrimSpace(s.config.SmsSDKAppID))
	request.SignName = common.StringPtr(strings.TrimSpace(s.config.SignName))
	request.TemplateId = common.StringPtr(strings.TrimSpace(s.config.TemplateID))
	request.PhoneNumberSet = common.StringPtrs([]string{strings.TrimSpace(phone)})
	request.TemplateParamSet = common.StringPtrs([]string{strings.TrimSpace(captcha), strconv.Itoa(expireMinutes)})

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
