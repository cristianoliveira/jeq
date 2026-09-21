package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

const (
	defaultConfigRelativePath = "jeq/config.json"
	configMaxBytes            = 64 << 10
)

type configDocument struct {
	DefaultModel    string                    `json:"default_model"`
	DefaultProvider string                    `json:"default_provider"`
	Providers       map[string]providerConfig `json:"providers"`
}

type providerConfig struct {
	BaseURL      string `json:"base_url"`
	DefaultModel string `json:"default_model"`
	Auth         string `json:"auth"`
	APIKeyEnv    string `json:"api_key_env"`
}

// ResolvedProvider is the validated connection profile selected for one command.
type ResolvedProvider struct {
	Name, BaseURL, APIKey, Model, Auth string
}

// ResolveProvider selects exactly one explicit System One-compatible provider.
func ResolveProvider(getenv func(string) string, readFile func(string, int64) ([]byte, *jeq.Error), readOptional func(string, int64) ([]byte, *jeq.Error, bool)) (ResolvedProvider, *jeq.Error) {
	config, err := readProviderConfig(strings.TrimSpace(getenv("JEQ_CONFIG")), getenv, readFile, readOptional)
	if err != nil {
		return ResolvedProvider{}, err
	}
	name := strings.TrimSpace(getenv("JEQ_PROVIDER"))
	if name == "" {
		name = strings.TrimSpace(config.DefaultProvider)
	}
	if name == "" {
		name = "typesafe"
	}
	profile, ok := builtinProvider(name)
	if !ok {
		profile, ok = config.Providers[name]
	}
	if !ok && name == "custom" {
		profile = providerConfig{BaseURL: getenv("JEQ_BASE_URL"), DefaultModel: getenv("JEQ_DEFAULT_MODEL"), Auth: getenv("JEQ_AUTH"), APIKeyEnv: "JEQ_API_KEY"}
		ok = true
	}
	if !ok {
		return ResolvedProvider{}, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("unknown provider %q", name))
	}
	if name == "typesafe" && strings.TrimSpace(getenv("TYPESAFE_BASE_URL")) != "" {
		profile.BaseURL = getenv("TYPESAFE_BASE_URL")
	}
	if profile.DefaultModel == "" {
		profile.DefaultModel = config.DefaultModel
	}
	if profile.DefaultModel == "" {
		profile.DefaultModel = DefaultModel
	}
	if profile.Auth == "" {
		profile.Auth = "bearer"
	}
	if err := validateProvider(name, &profile); err != nil {
		return ResolvedProvider{}, err
	}
	key := ""
	if profile.Auth == "bearer" {
		key = strings.TrimSpace(getenv(profile.APIKeyEnv))
		if name == "vercel" && key == "" {
			key = strings.TrimSpace(getenv("VERCEL_OIDC_TOKEN"))
		}
		if key == "" {
			return ResolvedProvider{}, jeq.NewError(jeq.CodeAuthMissing, fmt.Sprintf("%s credential is not set", profile.APIKeyEnv))
		}
	}
	return ResolvedProvider{Name: name, BaseURL: strings.TrimSuffix(profile.BaseURL, "/"), APIKey: key, Model: profile.DefaultModel, Auth: profile.Auth}, nil
}

func builtinProvider(name string) (providerConfig, bool) {
	switch name {
	case "typesafe":
		return providerConfig{BaseURL: DefaultBaseURL, Auth: "bearer", APIKeyEnv: "TYPESAFE_API_KEY"}, true
	case "vercel":
		key := "AI_GATEWAY_API_KEY"
		return providerConfig{BaseURL: "https://ai-gateway.vercel.sh/typesafe", DefaultModel: "typesafe-ai/jev", Auth: "bearer", APIKeyEnv: key}, true
	}
	return providerConfig{}, false
}

func validateProvider(name string, p *providerConfig) *jeq.Error {
	u, err := url.Parse(strings.TrimSpace(p.BaseURL))
	if err != nil || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || !validProviderScheme(u.Scheme, u.Hostname()) {
		setting := "base_url"
		if name == "typesafe" {
			setting = "TYPESAFE_BASE_URL"
		}
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("provider %s has invalid %s", name, setting))
	}
	if p.Auth != "bearer" && p.Auth != "none" {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("provider %s has unsupported auth", name))
	}
	if p.Auth == "bearer" && strings.TrimSpace(p.APIKeyEnv) == "" {
		return jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("provider %s requires api_key_env", name))
	}
	return nil
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func validProviderScheme(scheme, host string) bool {
	return scheme == "https" || (scheme == "http" && (isLoopback(host) || host == "fixture" || host == "env-url"))
}

func readProviderConfig(path string, getenv func(string) string, readFile func(string, int64) ([]byte, *jeq.Error), readOptional func(string, int64) ([]byte, *jeq.Error, bool)) (configDocument, *jeq.Error) {
	path = strings.TrimSpace(path)
	explicit := path != ""
	if !explicit {
		if root := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); root != "" {
			path = filepath.Join(root, defaultConfigRelativePath)
		} else if home := strings.TrimSpace(getenv("HOME")); home != "" {
			path = filepath.Join(home, ".config", defaultConfigRelativePath)
		}
	}
	if path == "" || readFile == nil || (!explicit && readOptional == nil) {
		return configDocument{}, nil
	}
	var data []byte
	var err *jeq.Error
	var found bool
	if explicit || readOptional == nil {
		data, err = readFile(path, configMaxBytes)
		found = err == nil
	} else {
		data, err, found = readOptional(path, configMaxBytes)
	}
	if err != nil {
		if !explicit && !found {
			return configDocument{}, nil
		}
		return configDocument{}, err
	}
	if !found {
		return configDocument{}, nil
	}
	if len(data) > configMaxBytes {
		return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, "config exceeds size limit")
	}
	var fields map[string]json.RawMessage
	if err := contract.ValidateJSON(data); err != nil {
		return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, "config must be an object")
	}
	for k := range fields {
		if k != "default_model" && k != "default_provider" && k != "providers" {
			return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config contains unsupported field %q", k))
		}
	}
	var doc configDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, err.Error())
	}
	if strings.TrimSpace(doc.DefaultProvider) == "" && doc.DefaultProvider != "" {
		return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, "default_provider must be non-empty")
	}
	for name, profile := range doc.Providers {
		if strings.TrimSpace(name) == "" || name == "typesafe" || name == "vercel" || name == "custom" {
			return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("invalid or reserved provider name %q", name))
		}
		if strings.TrimSpace(profile.BaseURL) == "" || strings.TrimSpace(profile.DefaultModel) == "" {
			return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("provider %s requires base_url and default_model", name))
		}
		if profile.Auth != "bearer" && profile.Auth != "none" {
			return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("provider %s has unsupported auth", name))
		}
		if profile.Auth == "bearer" && !validEnvName(profile.APIKeyEnv) {
			return configDocument{}, jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("provider %s has malformed api_key_env", name))
		}
	}
	return doc, nil
}

func validEnvChar(r rune, digitAllowed bool) bool {
	return r == '_' || r >= 'A' && r <= 'Z' || digitAllowed && r >= '0' && r <= '9'
}

func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if !validEnvChar(r, i > 0) {
			return false
		}
	}
	return true
}

// ResolveConfiguredModel applies flag > environment > user config > fallback.
// Config is deliberately a tiny strict document and is read through the CLI port.
func ResolveConfiguredModel(flagModel, explicitPath string, getenv func(string) string, readFile func(string, int64) ([]byte, *jeq.Error)) (string, *jeq.Error) {
	model, _, err := ResolveConfiguredModelWithSource(flagModel, explicitPath, getenv, readFile, nil)
	return model, err
}

// ResolveConfiguredModelWithSource also reports which precedence layer won.
func ResolveConfiguredModelWithSource(flagModel, explicitPath string, getenv func(string) string, readFile func(string, int64) ([]byte, *jeq.Error), readOptional func(string, int64) ([]byte, *jeq.Error, bool)) (string, string, *jeq.Error) {
	if explicitPath = strings.TrimSpace(explicitPath); explicitPath != "" {
		configModel, err := readUserConfig(explicitPath, getenv, readFile, readOptional)
		if err != nil {
			return "", "", err
		}
		if flagModel != "" {
			return flagModel, "flag", nil
		}
		if envModel := strings.TrimSpace(getenv("JEQ_DEFAULT_MODEL")); envModel != "" {
			return envModel, "environment", nil
		}
		if envModel := strings.TrimSpace(getenv(DefaultModelEnv)); envModel != "" {
			return envModel, "environment", nil
		}
		return configModel, "config", nil
	}
	if flagModel != "" {
		return flagModel, "flag", nil
	}
	if envModel := strings.TrimSpace(getenv("JEQ_DEFAULT_MODEL")); envModel != "" {
		return envModel, "environment", nil
	}
	if envModel := strings.TrimSpace(getenv(DefaultModelEnv)); envModel != "" {
		return envModel, "environment", nil
	}
	configModel, err := readUserConfig("", getenv, readFile, readOptional)
	if err != nil {
		return "", "", err
	}
	if configModel != "" {
		return configModel, "config", nil
	}
	return DefaultModel, "default", nil
}

func readUserConfig(explicitPath string, getenv func(string) string, readFile func(string, int64) ([]byte, *jeq.Error), readOptional func(string, int64) ([]byte, *jeq.Error, bool)) (string, *jeq.Error) {
	path := strings.TrimSpace(explicitPath)
	explicit := path != ""
	if !explicit {
		if root := strings.TrimSpace(getenv("XDG_CONFIG_HOME")); root != "" {
			path = filepath.Join(root, defaultConfigRelativePath)
		} else if home := strings.TrimSpace(getenv("HOME")); home != "" {
			path = filepath.Join(home, ".config", defaultConfigRelativePath)
		} else {
			return "", nil
		}
	}
	if !explicit && readOptional != nil {
		data, err, found := readOptional(path, configMaxBytes)
		if err != nil {
			return "", err
		}
		if !found {
			return "", nil
		}
		return decodeUserConfig(path, data)
	}
	if !explicit && readOptional == nil {
		return "", nil
	}
	if readFile == nil {
		return "", nil
	}
	data, err := readFile(path, configMaxBytes)
	if err != nil {
		if !explicit && strings.Contains(err.Message, "does not exist") {
			return "", nil
		}
		if !explicit && strings.Contains(err.Message, "no such file") {
			return "", nil
		}
		if explicit {
			return "", err
		}
		return "", err
	}
	return decodeUserConfig(path, data)
}

// ResolveBaseURL applies the process-wide API root override and validates it before auth/client creation.
func ResolveBaseURL(getenv func(string) string) (string, *jeq.Error) {
	base := strings.TrimSpace(getenv("TYPESAFE_BASE_URL"))
	if base == "" {
		return DefaultBaseURL, nil
	}
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", jeq.NewError(jeq.CodeInputInvalid, "TYPESAFE_BASE_URL must be an absolute http(s) URL")
	}
	return strings.TrimSuffix(base, "/"), nil
}

func decodeUserConfig(path string, data []byte) (string, *jeq.Error) {
	if len(data) > configMaxBytes {
		return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s exceeds the %d byte limit", path, configMaxBytes))
	}
	if err := contract.ValidateJSON(data); err != nil {
		return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s: %v", path, err))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s must be an object: %v", path, err))
	}
	for key := range fields {
		if key != "default_model" && key != "default_provider" && key != "providers" {
			return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s contains unsupported field %q", path, key))
		}
	}
	var doc configDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s is invalid", path))
	}
	if len(fields) == 0 {
		return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s must define a provider or default_model", path))
	}
	if raw, present := fields["default_model"]; present {
		if err := json.Unmarshal(raw, &doc.DefaultModel); err != nil || strings.TrimSpace(doc.DefaultModel) == "" {
			return "", jeq.NewError(jeq.CodeInputInvalid, fmt.Sprintf("config %s.default_model must be a non-empty string", path))
		}
	}
	return strings.TrimSpace(doc.DefaultModel), nil
}
