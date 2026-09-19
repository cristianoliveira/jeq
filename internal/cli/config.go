package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/contract"
	"github.com/cristianoliveira/gev/internal/domain/gev"
)

const (
	defaultConfigRelativePath = "gev/config.json"
	configMaxBytes            = 64 << 10
)

type configDocument struct {
	DefaultModel string `json:"default_model"`
}

// ResolveConfiguredModel applies flag > environment > user config > fallback.
// Config is deliberately a tiny strict document and is read through the CLI port.
func ResolveConfiguredModel(flagModel, explicitPath string, getenv func(string) string, readFile func(string, int64) ([]byte, *gev.Error)) (string, *gev.Error) {
	model, _, err := ResolveConfiguredModelWithSource(flagModel, explicitPath, getenv, readFile, nil)
	return model, err
}

// ResolveConfiguredModelWithSource also reports which precedence layer won.
func ResolveConfiguredModelWithSource(flagModel, explicitPath string, getenv func(string) string, readFile func(string, int64) ([]byte, *gev.Error), readOptional func(string, int64) ([]byte, *gev.Error, bool)) (string, string, *gev.Error) {
	configModel, err := readUserConfig(explicitPath, getenv, readFile, readOptional)
	if err != nil {
		return "", "", err
	}
	if flagModel != "" {
		return flagModel, "flag", nil
	}
	if envModel := strings.TrimSpace(getenv(DefaultModelEnv)); envModel != "" {
		return envModel, "environment", nil
	}
	if configModel != "" {
		return configModel, "config", nil
	}
	return DefaultModel, "default", nil
}

func readUserConfig(explicitPath string, getenv func(string) string, readFile func(string, int64) ([]byte, *gev.Error), readOptional func(string, int64) ([]byte, *gev.Error, bool)) (string, *gev.Error) {
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

func decodeUserConfig(path string, data []byte) (string, *gev.Error) {
	if err := contract.ValidateJSON(data); err != nil {
		return "", gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("config %s: %v", path, err))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return "", gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("config %s must be an object: %v", path, err))
	}
	for key := range fields {
		if key != "default_model" {
			return "", gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("config %s contains unsupported field %q", path, key))
		}
	}
	var doc configDocument
	if err := json.Unmarshal(data, &doc); err != nil || strings.TrimSpace(doc.DefaultModel) == "" {
		return "", gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("config %s.default_model must be a non-empty string", path))
	}
	return strings.TrimSpace(doc.DefaultModel), nil
}
