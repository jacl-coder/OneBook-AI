package verify

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

const (
	purposeSignup        = "signup"
	purposeLogin         = "login"
	purposePasswordReset = "password_reset"
)

// Message contains the provider-neutral data needed to deliver a verification code.
type Message struct {
	Channel     domain.IdentityType
	Identifier  string
	Purpose     string
	Code        string
	ExpiresIn   int
	ResendAfter int
}

// Sender delivers verification messages across configured channels.
type Sender interface {
	SendVerification(context.Context, Message) error
}

// Config holds provider selection and credentials for verification delivery.
type Config struct {
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

type emailMessage struct {
	To        string
	Purpose   string
	Code      string
	ExpiresIn int
}

type smsMessage struct {
	To          string
	Purpose     string
	Code        string
	ExpiresIn   int
	ResendAfter int
}

type emailProvider interface {
	SendVerificationEmail(context.Context, emailMessage) error
}

type smsProvider interface {
	SendVerificationSMS(context.Context, smsMessage) error
}

type emailProviderFactory func(Config) (emailProvider, error)

type smsProviderFactory func(Config) (smsProvider, error)

var emailProviderFactories = map[string]emailProviderFactory{
	"console": newConsoleEmailProvider,
	"resend":  newResendEmailProvider,
}

var smsProviderFactories = map[string]smsProviderFactory{
	"console": newConsoleSMSProvider,
	"aliyun":  newAliyunSMSProvider,
}

type compositeSender struct {
	email emailProvider
	sms   smsProvider
}

// NewSender creates a verification sender from provider config.
func NewSender(cfg Config) (Sender, error) {
	email, err := newEmailProvider(cfg)
	if err != nil {
		return nil, err
	}
	sms, err := newSMSProvider(cfg)
	if err != nil {
		return nil, err
	}
	return compositeSender{email: email, sms: sms}, nil
}

func newEmailProvider(cfg Config) (emailProvider, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	if provider == "" {
		provider = "console"
	}
	factory, ok := emailProviderFactories[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported email provider %q", provider)
	}
	return factory(cfg)
}

func newSMSProvider(cfg Config) (smsProvider, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.SMSProvider))
	if provider == "" {
		provider = "console"
	}
	factory, ok := smsProviderFactories[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported sms provider %q", provider)
	}
	return factory(cfg)
}

func (s compositeSender) SendVerification(ctx context.Context, msg Message) error {
	switch msg.Channel {
	case domain.IdentityEmail:
		return s.email.SendVerificationEmail(ctx, emailMessage{
			To:        msg.Identifier,
			Purpose:   msg.Purpose,
			Code:      msg.Code,
			ExpiresIn: msg.ExpiresIn,
		})
	case domain.IdentityPhone:
		return s.sms.SendVerificationSMS(ctx, smsMessage{
			To:          msg.Identifier,
			Purpose:     msg.Purpose,
			Code:        msg.Code,
			ExpiresIn:   msg.ExpiresIn,
			ResendAfter: msg.ResendAfter,
		})
	default:
		return fmt.Errorf("unsupported verification channel %q", msg.Channel)
	}
}

type consoleEmailProvider struct{}

func newConsoleEmailProvider(Config) (emailProvider, error) {
	return consoleEmailProvider{}, nil
}

func (consoleEmailProvider) SendVerificationEmail(ctx context.Context, msg emailMessage) error {
	util.LoggerFromContext(ctx).Info(
		"verification_code_created",
		"channel", string(domain.IdentityEmail),
		"identifier", maskIdentifier(domain.IdentityEmail, msg.To),
		"purpose", msg.Purpose,
	)
	return nil
}

type consoleSMSProvider struct{}

func newConsoleSMSProvider(Config) (smsProvider, error) {
	return consoleSMSProvider{}, nil
}

func (consoleSMSProvider) SendVerificationSMS(ctx context.Context, msg smsMessage) error {
	util.LoggerFromContext(ctx).Info(
		"verification_code_created",
		"channel", string(domain.IdentityPhone),
		"identifier", maskIdentifier(domain.IdentityPhone, msg.To),
		"purpose", msg.Purpose,
	)
	return nil
}

type resendEmailProvider struct {
	apiKey string
	from   string
	client http.Client
}

func newResendEmailProvider(cfg Config) (emailProvider, error) {
	if strings.TrimSpace(cfg.ResendAPIKey) == "" || strings.TrimSpace(cfg.ResendFrom) == "" {
		return nil, errors.New("resend provider requires RESEND_API_KEY and RESEND_FROM")
	}
	return &resendEmailProvider{
		apiKey: strings.TrimSpace(cfg.ResendAPIKey),
		from:   strings.TrimSpace(cfg.ResendFrom),
		client: http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (s *resendEmailProvider) SendVerificationEmail(ctx context.Context, msg emailMessage) error {
	payload := map[string]any{
		"from":    s.from,
		"to":      []string{msg.To},
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

type aliyunSMSProvider struct {
	signName      string
	templateCodes map[string]string
	client        *dypnsapi.Client
}

func newAliyunSMSProvider(cfg Config) (smsProvider, error) {
	if strings.TrimSpace(cfg.AliyunAccessKeyID) == "" ||
		strings.TrimSpace(cfg.AliyunAccessKeySecret) == "" ||
		strings.TrimSpace(cfg.AliyunSMSSignName) == "" {
		return nil, errors.New("aliyun sms provider requires ALIYUN_ACCESS_KEY_ID, ALIYUN_ACCESS_KEY_SECRET, and ALIYUN_SMS_SIGN_NAME")
	}
	aliyunClient, err := newAliyunPNVSClient(cfg.AliyunAccessKeyID, cfg.AliyunAccessKeySecret)
	if err != nil {
		return nil, err
	}
	return &aliyunSMSProvider{
		signName: strings.TrimSpace(cfg.AliyunSMSSignName),
		templateCodes: map[string]string{
			purposeSignup:          firstNonEmpty(cfg.AliyunSMSSignupLoginTemplate, "100001"),
			purposeLogin:           firstNonEmpty(cfg.AliyunSMSSignupLoginTemplate, "100001"),
			purposePasswordReset:   firstNonEmpty(cfg.AliyunSMSPasswordResetTemplate, "100003"),
			"change_phone":         firstNonEmpty(cfg.AliyunSMSChangePhoneTemplate, "100002"),
			"bind_phone":           firstNonEmpty(cfg.AliyunSMSBindPhoneTemplate, "100004"),
			"verify_binding_phone": firstNonEmpty(cfg.AliyunSMSVerifyBindingTemplate, "100005"),
		},
		client: aliyunClient,
	}, nil
}

func (s *aliyunSMSProvider) SendVerificationSMS(ctx context.Context, msg smsMessage) error {
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
		PhoneNumber:   tea.String(strings.TrimPrefix(msg.To, "+86")),
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

func (s *aliyunSMSProvider) templateCodeForPurpose(purpose string) string {
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

func maskIdentifier(channel domain.IdentityType, identifier string) string {
	if channel == domain.IdentityPhone {
		return maskPhone(identifier)
	}
	return maskEmail(identifier)
}

func maskEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	domain := parts[1]
	switch len(local) {
	case 0:
		return "***@" + domain
	case 1:
		return local + "***@" + domain
	case 2:
		return local[:1] + "***@" + domain
	default:
		return local[:1] + "***" + local[len(local)-1:] + "@" + domain
	}
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
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
