package tencentsms

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	authphone "github.com/darren-you/auth_service/template_server/pkg/phone"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const defaultRegion = "ap-guangzhou"
const defaultAppKeyEndpoint = "https://yun.tim.qq.com/v5/tlssmssvr/sendsms"

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
	AppKeyEndpoint string
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
	if strings.TrimSpace(s.config.AppKey) != "" {
		return s.sendCaptchaWithAppKey(message)
	}
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

func (s *Sender) sendCaptchaWithAppKey(message authphone.CaptchaMessage) error {
	templateID, params := s.resolveTemplate(message.Scene)
	if strings.TrimSpace(templateID) == "" {
		return fmt.Errorf("tencent sms template id is required for scene %s", authphone.NormalizeCaptchaScene(message.Scene))
	}
	tplID, err := strconv.Atoi(strings.TrimSpace(templateID))
	if err != nil {
		return fmt.Errorf("tencent sms template id must be numeric: %w", err)
	}

	appID := strings.TrimSpace(s.config.SmsSDKAppID)
	appKey := strings.TrimSpace(s.config.AppKey)
	phone := normalizeMainlandMobile(message.Phone)
	now := time.Now().Unix()
	random := strconv.FormatInt(time.Now().UnixNano()%1000000000, 10)
	sig := appKeySignature(appKey, random, now, phone)

	endpoint := strings.TrimSpace(s.config.AppKeyEndpoint)
	if endpoint == "" {
		endpoint = defaultAppKeyEndpoint
	}
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse tencent sms endpoint: %w", err)
	}
	query := parsedURL.Query()
	query.Set("sdkappid", appID)
	query.Set("random", random)
	parsedURL.RawQuery = query.Encode()

	body := appKeySendRequest{
		Tel: appKeyPhone{
			NationCode: "86",
			Mobile:     phone,
		},
		Sign:     strings.TrimSpace(s.config.SignName),
		Template: tplID,
		Params:   resolveTemplateParamValues(params, message),
		Sig:      sig,
		Time:     now,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal tencent sms request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	request, err := http.NewRequest(http.MethodPost, parsedURL.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create tencent sms request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send tencent sms request: %w", err)
	}
	defer response.Body.Close()

	content, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read tencent sms response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("tencent sms http status %d: %s", response.StatusCode, strings.TrimSpace(string(content)))
	}

	var result appKeySendResponse
	if err := json.Unmarshal(content, &result); err != nil {
		return fmt.Errorf("parse tencent sms response: %w", err)
	}
	if result.Result != 0 {
		return fmt.Errorf("failed to send sms: %d - %s", result.Result, result.ErrMsg)
	}
	return nil
}

type appKeyPhone struct {
	NationCode string `json:"nationcode"`
	Mobile     string `json:"mobile"`
}

type appKeySendRequest struct {
	Tel      appKeyPhone `json:"tel"`
	Sign     string      `json:"sign"`
	Template int         `json:"tpl_id"`
	Params   []string    `json:"params"`
	Sig      string      `json:"sig"`
	Time     int64       `json:"time"`
	Extend   string      `json:"extend,omitempty"`
	Ext      string      `json:"ext,omitempty"`
}

type appKeySendResponse struct {
	Result int    `json:"result"`
	ErrMsg string `json:"errmsg"`
}

func appKeySignature(appKey string, random string, timestamp int64, mobile string) string {
	source := fmt.Sprintf("appkey=%s&random=%s&time=%d&mobile=%s", appKey, random, timestamp, mobile)
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func normalizeMainlandMobile(phone string) string {
	normalized := strings.TrimSpace(phone)
	normalized = strings.TrimPrefix(normalized, "+86")
	if len(normalized) == 13 && strings.HasPrefix(normalized, "86") {
		normalized = strings.TrimPrefix(normalized, "86")
	}
	return normalized
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
