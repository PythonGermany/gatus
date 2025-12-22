package threemagateway

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/TwiN/gatus/v5/alerting/alert"
	"github.com/TwiN/gatus/v5/client"
	"github.com/TwiN/gatus/v5/config/endpoint"
	"gopkg.in/yaml.v3"
)

// TODO#1464: Add tests

const (
	defaultApiUrl = "https://msgapi.threema.ch"
	defaultMode   = "basic"
)

var (
	ErrModeInvalid        = fmt.Errorf("invalid mode, must be one of: %s", joinKeys(modes, ", "))
	ErrNotImplementedMode = errors.New("the specified mode is not implemented yet")
	modes                 = map[string]Mode{
		"basic": ModeBasic,
		"e2ee":  ModeE2EE,
	}

	ErrInvalidRecipientType = fmt.Errorf("invalid recipient type, must be one of: %v", joinKeys(recipientTypes, ", "))
	recipientTypes          = map[string]RecipientType{
		"id":    RecipientTypeID,
		"phone": RecipientTypePhone,
		"email": RecipientTypeEmail,
	}

	ErrApiKeyMissing = errors.New("auth-secret is required")
)

func joinKeys[V any](m map[string]V, separator string) string {
	return strings.Join(slices.Collect(maps.Keys(m)), separator)
}

type Mode int

const (
	ModeInvalid Mode = iota
	ModeBasic
	ModeE2EE
)

func parseMode(s string) Mode {
	if val, ok := modes[s]; ok {
		return val
	}
	return ModeInvalid
}

type RecipientType int

const (
	RecipientTypeInvalid RecipientType = iota
	RecipientTypeID
	RecipientTypePhone
	RecipientTypeEmail
)

type Config struct {
	ApiUrl        string `yaml:"api-url"`
	Mode          string `yaml:"mode"`
	SenderID      string `yaml:"sender-id"`
	Recipient     string `yaml:"recipient"`
	ApiAuthSecret string `yaml:"auth-secret"`

	sendMode        Mode          `yaml:"-"`
	recipientType   RecipientType `yaml:"-"`
	parsedRecipient string        `yaml:"-"`
}

func parseRecipientType(s string) RecipientType {
	if val, ok := recipientTypes[s]; ok {
		return val
	}
	return RecipientTypeInvalid
}

func parseRecipient(s string) (RecipientType, string, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return RecipientTypeInvalid, "", errors.New("recipient must be in the format '<type>:<value>'")
	}
	recipientType := parseRecipientType(parts[0])
	if recipientType == RecipientTypeInvalid {
		return RecipientTypeInvalid, "", ErrInvalidRecipientType
	}
	return recipientType, parts[1], nil
}

func validateRecipient(recipientType RecipientType, recipient string) error {
	if len(recipient) == 0 {
		return errors.New("recipient value cannot be empty")
	}
	switch recipientType {
	case RecipientTypeID:
		if len(recipient) != 8 {
			return errors.New("recipient ID must be 8 characters long")
		}
	case RecipientTypePhone:
		// Basic validation for phone number # TODO#1464: improve phone number validation
		if !strings.HasPrefix(recipient, "+") || len(recipient) < 8 {
			return errors.New("invalid phone number format")
		}
	case RecipientTypeEmail:
		// Basic validation for email address // TODO#1464: improve email validation
		if !strings.Contains(recipient, "@") {
			return errors.New("invalid email address format")
		}
	default:
		return ErrInvalidRecipientType
	}
	return nil
}

func (cfg *Config) Validate() error {
	// Validate API URL
	if len(cfg.ApiUrl) == 0 {
		cfg.ApiUrl = defaultApiUrl
	}

	// Validate Mode
	if len(cfg.Mode) == 0 {
		cfg.Mode = defaultMode
	}
	cfg.sendMode = parseMode(cfg.Mode)
	if cfg.sendMode == ModeInvalid {
		return ErrModeInvalid
	} else if cfg.sendMode != ModeBasic {
		return ErrNotImplementedMode
	}

	// Validate Recipient
	var err error
	cfg.recipientType, cfg.parsedRecipient, err = parseRecipient(cfg.Recipient)
	if err != nil {
		return err
	}
	if err := validateRecipient(cfg.recipientType, cfg.parsedRecipient); err != nil {
		return err
	}

	// Validate API Key
	if len(cfg.ApiAuthSecret) == 0 {
		return ErrApiKeyMissing
	}
	return nil
}

func (cfg *Config) Merge(override *Config) {
	if len(override.ApiUrl) > 0 {
		cfg.ApiUrl = override.ApiUrl
	}
	if len(override.Mode) > 0 {
		cfg.Mode = override.Mode
		cfg.sendMode = ModeInvalid
	}
	if len(override.SenderID) > 0 {
		cfg.SenderID = override.SenderID
	}
	if len(override.Recipient) > 0 {
		cfg.Recipient = override.Recipient
		cfg.recipientType = RecipientTypeInvalid
	}
	if len(override.ApiAuthSecret) > 0 {
		cfg.ApiAuthSecret = override.ApiAuthSecret
	}
}

type AlertProvider struct {
	DefaultConfig Config       `yaml:",inline"`
	DefaultAlert  *alert.Alert `yaml:"default-alert,omitempty"`
	Overrides     []Override   `yaml:"overrides,omitempty"`
}

type Override struct {
	Group  string `yaml:"group"`
	Config `yaml:",inline"`
}

func (provider *AlertProvider) Validate() error {
	return provider.DefaultConfig.Validate()
}

func (provider *AlertProvider) Send(ep *endpoint.Endpoint, alert *alert.Alert, result *endpoint.Result, resolved bool) error {
	cfg, err := provider.GetConfig(ep.Group, alert)
	if err != nil {
		return err
	}
	body := provider.buildMessageBody(ep, alert, result, resolved)
	request, err := provider.prepareRequest(cfg, body)
	if err != nil {
		return err
	}
	response, err := client.GetHTTPClient(nil).Do(request)
	if err != nil {
		return err
	}
	return handleResponse(cfg, response)
}

func (provider *AlertProvider) buildMessageBody(ep *endpoint.Endpoint, alert *alert.Alert, result *endpoint.Result, resolved bool) string {
	var body string
	if resolved {
		body = fmt.Sprintf("✅ Alert resolved for endpoint '%s' after passing %d checks.", ep.Name, alert.SuccessThreshold)
	} else {
		body = fmt.Sprintf("🚨 Alert triggered for endpoint '%s' after failing %d checks.\n\nConditions:\n", ep.Name, alert.FailureThreshold)
		for _, conditionResult := range result.ConditionResults {
			var icon rune
			if conditionResult.Success {
				icon = '✓'
			} else {
				icon = '✗'
			}
			body += fmt.Sprintf("- %c %s\n", icon, conditionResult.Condition)
		}
		if len(result.Errors) > 0 {
			body += "\nErrors:\n"
			for _, err := range result.Errors {
				body += fmt.Sprintf("- ✗ %s\n", err)
			}
		}
	}
	return body
}

func (provider *AlertProvider) prepareRequest(cfg *Config, body string) (*http.Request, error) {
	requestUrl := cfg.ApiUrl
	switch cfg.sendMode {
	case ModeBasic:
		requestUrl += "/send_simple"
	default:
		return nil, ErrNotImplementedMode
	}

	data := url.Values{}
	data.Add("from", cfg.SenderID)
	var toKey string
	switch cfg.recipientType {
	case RecipientTypeID:
		toKey = "to"
	case RecipientTypePhone:
		toKey = "phone"
	case RecipientTypeEmail:
		toKey = "email"
	default:
		return nil, ErrInvalidRecipientType
	}
	data.Add(toKey, cfg.parsedRecipient)
	data.Add("text", body)
	data.Add("secret", cfg.ApiAuthSecret)

	request, err := http.NewRequest(http.MethodPost, requestUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request, nil
}

func handleResponse(cfg *Config, response *http.Response) error {
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("%s: Invalid recipient or Threema Gateway account not set up for %s mode", response.Status, cfg.Mode)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: Invalid auth-secret or sender-id", response.Status)
	case http.StatusPaymentRequired:
		return fmt.Errorf("%s: Insufficient credits to send message", response.Status)
	case http.StatusNotFound:
		return fmt.Errorf("%s: Recipient could not be found", response.Status)
	default:
		return fmt.Errorf("Response: %s", response.Status)
	}
}

func (provider *AlertProvider) GetDefaultAlert() *alert.Alert {
	return provider.DefaultAlert
}

// GetConfig returns the configuration for the provider with the overrides applied
func (provider *AlertProvider) GetConfig(group string, alert *alert.Alert) (*Config, error) {
	cfg := provider.DefaultConfig
	// Handle group overrides
	if len(provider.Overrides) > 0 {
		for _, override := range provider.Overrides {
			if group == override.Group {
				cfg.Merge(&override.Config)
				break
			}
		}
	}
	// Handle alert overrides
	if len(alert.ProviderOverride) > 0 {
		overrideConfig := Config{}
		if err := yaml.Unmarshal(alert.ProviderOverrideAsBytes(), &overrideConfig); err != nil {
			return nil, err
		}
		cfg.Merge(&overrideConfig)
	}
	return &cfg, cfg.Validate()
}

func (provider *AlertProvider) ValidateOverrides(group string, alert *alert.Alert) error {
	_, err := provider.GetConfig(group, alert)
	return err
}
