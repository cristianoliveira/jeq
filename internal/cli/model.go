package cli

import "os"

// DefaultModel is the documented fallback when no model is configured.
const DefaultModel = "jev-latest"

// DefaultModelEnv is the legacy environment variable holding the default model.
// JEQ_DEFAULT_MODEL takes precedence when selecting composed request models.
const DefaultModelEnv = "TYPESAFE_DEFAULT_MODEL"

// ResolveModel applies the legacy model-only precedence: --model, then
// TYPESAFE_DEFAULT_MODEL, then jev-latest. Network commands use
// ResolveConfiguredModelWithSource to include provider and user config.
func ResolveModel(flagModel string, getenv func(string) string) string {
	if flagModel != "" {
		return flagModel
	}
	if v := getenv(DefaultModelEnv); v != "" {
		return v
	}
	return DefaultModel
}

// ResolveModelFromEnv is ResolveModel against the process environment.
func ResolveModelFromEnv(flagModel string) string {
	return ResolveModel(flagModel, os.Getenv)
}
