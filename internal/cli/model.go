package cli

import "os"

// DefaultModel is the documented fallback when no model is configured.
const DefaultModel = "jev-latest"

// DefaultModelEnv is the environment variable holding the account's
// preferred default model.
const DefaultModelEnv = "TYPESAFE_DEFAULT_MODEL"

// ResolveModel applies the documented precedence (ADR 0001 Configuration):
// --model, then TYPESAFE_DEFAULT_MODEL, then jev-latest. Resolution lives
// in the shell layer because it reads the environment; the domain receives
// the resolved model explicitly.
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
