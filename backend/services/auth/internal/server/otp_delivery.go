package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dypnsapi "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	utilservice "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
)

type verificationMessage struct {
	Channel     domain.IdentityType
	Identifier  string
	Purpose     string
	Code        string
	ExpiresIn   int
	ResendAfter int
}

type verificationSender interface {
	SendVerification(context.Context, verificationMessage) error
}

type senderConfig struct {
	EmailProvider                  string
	SMSProvider                    string
	ResendAPIKey                   string
	ResendFrom                     string
	AliyunAccessKeyID              string
	AliyunAccessKeySecret          string
	AliyunSMSSignName              string
	AliyunSMSSignupLoginTemplate   string
	AliyunSMSPasswordResetTemplate string
	AliyunSMSChangePhoneTemplate   string
	AliyunSMSBindPhoneTemplate     string
	AliyunSMSVerifyBindingTemplate string
}

type compositeVerificationSender struct {
	email verificationSender
	sms   verificationSender
}

func newVerificationSender(cfg senderConfig) (verificationSender, error) {
	emailProvider := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	if emailProvider == "" {
		emailProvider = "console"
	}
	smsProvider := strings.ToLower(strings.TrimSpace(cfg.SMSProvider))
	if smsProvider == "" {
		smsProvider = "console"
	}

	var email verificationSender
	switch emailProvider {
	case "console":
		email = consoleVerificationSender{}
	case "resend":
		if strings.TrimSpace(cfg.ResendAPIKey) == "" || strings.TrimSpace(cfg.ResendFrom) == "" {
			return nil, errors.New("resend provider requires RESEND_API_KEY and RESEND_FROM")
		}
		email = &resendVerificationSender{
			apiKey: strings.TrimSpace(cfg.ResendAPIKey),
			from:   strings.TrimSpace(cfg.ResendFrom),
			client: http.Client{Timeout: 5 * time.Second},
		}
	default:
		return nil, fmt.Errorf("unsupported email provider %q", emailProvider)
	}

	var sms verificationSender
	switch smsProvider {
	case "console":
		sms = consoleVerificationSender{}
	case "aliyun":
		if strings.TrimSpace(cfg.AliyunAccessKeyID) == "" ||
			strings.TrimSpace(cfg.AliyunAccessKeySecret) == "" ||
			strings.TrimSpace(cfg.AliyunSMSSignName) == "" {
			return nil, errors.New("aliyun sms provider requires ALIYUN_ACCESS_KEY_ID, ALIYUN_ACCESS_KEY_SECRET, and ALIYUN_SMS_SIGN_NAME")
		}
		aliyunClient, err := newAliyunPNVSClient(cfg.AliyunAccessKeyID, cfg.AliyunAccessKeySecret)
		if err != nil {
			return nil, err
		}
		sms = &aliyunSMSSender{
			signName: strings.TrimSpace(cfg.AliyunSMSSignName),
			templateCodes: map[string]string{
				verificationPurposeSignup:        firstNonEmpty(cfg.AliyunSMSSignupLoginTemplate, "100001"),
				verificationPurposeLogin:         firstNonEmpty(cfg.AliyunSMSSignupLoginTemplate, "100001"),
				verificationPurposePasswordReset: firstNonEmpty(cfg.AliyunSMSPasswordResetTemplate, "100003"),
				"change_phone":                   firstNonEmpty(cfg.AliyunSMSChangePhoneTemplate, "100002"),
				"bind_phone":                     firstNonEmpty(cfg.AliyunSMSBindPhoneTemplate, "100004"),
				"verify_binding_phone":           firstNonEmpty(cfg.AliyunSMSVerifyBindingTemplate, "100005"),
			},
			client: aliyunClient,
		}
	default:
		return nil, fmt.Errorf("unsupported sms provider %q", smsProvider)
	}
	return compositeVerificationSender{email: email, sms: sms}, nil
}

func (s compositeVerificationSender) SendVerification(ctx context.Context, msg verificationMessage) error {
	switch msg.Channel {
	case domain.IdentityEmail:
		return s.email.SendVerification(ctx, msg)
	case domain.IdentityPhone:
		return s.sms.SendVerification(ctx, msg)
	default:
		return fmt.Errorf("unsupported verification channel %q", msg.Channel)
	}
}

type consoleVerificationSender struct{}

func (consoleVerificationSender) SendVerification(ctx context.Context, msg verificationMessage) error {
	util.LoggerFromContext(ctx).Info(
		"verification_code_created",
		"channel", string(msg.Channel),
		"identifier", maskIdentifier(msg.Channel, msg.Identifier),
		"purpose", msg.Purpose,
	)
	return nil
}

type resendVerificationSender struct {
	apiKey string
	from   string
	client http.Client
}

func (s *resendVerificationSender) SendVerification(ctx context.Context, msg verificationMessage) error {
	payload := map[string]any{
		"from":    s.from,
		"to":      []string{msg.Identifier},
		"subject": "Your OneBook AI verification code",
		"text":    fmt.Sprintf("Your OneBook AI verification code is %s. It expires in 10 minutes.", msg.Code),
		"html":    fmt.Sprintf("<p>Your OneBook AI verification code is <strong>%s</strong>.</p><p>It expires in 10 minutes.</p>", msg.Code),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "onebook-ai-auth/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type aliyunSMSSender struct {
	signName      string
	templateCodes map[string]string
	client        *dypnsapi.Client
}

func (s *aliyunSMSSender) SendVerification(ctx context.Context, msg verificationMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	expiresIn := positiveOrDefault(msg.ExpiresIn, 600)
	resendAfter := positiveOrDefault(msg.ResendAfter, 60)
	templateParam, err := json.Marshal(map[string]string{
		"code": msg.Code,
		"min":  strconv.Itoa(secondsToDisplayMinutes(expiresIn)),
	})
	if err != nil {
		return err
	}
	request := &dypnsapi.SendSmsVerifyCodeRequest{
		CountryCode:   tea.String("86"),
		Interval:      tea.Int64(int64(resendAfter)),
		OutId:         tea.String(util.NewID()),
		PhoneNumber:   tea.String(strings.TrimPrefix(msg.Identifier, "+86")),
		SignName:      tea.String(s.signName),
		TemplateCode:  tea.String(s.templateCodeForPurpose(msg.Purpose)),
		TemplateParam: tea.String(string(templateParam)),
		ValidTime:     tea.Int64(int64(expiresIn)),
	}
	runtime := &utilservice.RuntimeOptions{}
	resp, err := s.client.SendSmsVerifyCodeWithOptions(request, runtime)
	if err != nil {
		return err
	}
	if resp == nil || resp.Body == nil {
		return errors.New("aliyun pnvs sms verify send failed: empty response")
	}
	if !strings.EqualFold(tea.StringValue(resp.Body.Code), "OK") || !tea.BoolValue(resp.Body.Success) {
		return fmt.Errorf("aliyun pnvs sms verify send failed: request_id=%s code=%s message=%s", tea.StringValue(resp.Body.RequestId), tea.StringValue(resp.Body.Code), tea.StringValue(resp.Body.Message))
	}
	return nil
}

func (s *aliyunSMSSender) templateCodeForPurpose(purpose string) string {
	if code := strings.TrimSpace(s.templateCodes[strings.TrimSpace(purpose)]); code != "" {
		return code
	}
	return "100001"
}

func newAliyunPNVSClient(accessKeyID, accessKeySecret string) (*dypnsapi.Client, error) {
	cred, err := credential.NewCredential(&credential.Config{
		Type:            tea.String("access_key"),
		AccessKeyId:     tea.String(strings.TrimSpace(accessKeyID)),
		AccessKeySecret: tea.String(strings.TrimSpace(accessKeySecret)),
	})
	if err != nil {
		return nil, err
	}
	return dypnsapi.NewClient(&openapi.Config{
		Credential: cred,
		Endpoint:   tea.String("dypnsapi.aliyuncs.com"),
		RegionId:   tea.String("cn-shanghai"),
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func secondsToDisplayMinutes(seconds int) int {
	if seconds <= 0 {
		return 1
	}
	minutes := seconds / 60
	if seconds%60 != 0 {
		minutes++
	}
	if minutes <= 0 {
		return 1
	}
	return minutes
}
