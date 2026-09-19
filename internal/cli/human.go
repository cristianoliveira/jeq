package cli

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

func writeHome(w io.Writer, doc homeDocument) error {
	_, err := fmt.Fprintf(w, "gev\n%s\nCredentials: %s\nDefault model: %s (%s)\nCommands: %s\nNext: gev examples\n", doc.Purpose, readiness(doc.CredentialReady), doc.DefaultModel, doc.DefaultModelSource, strings.Join(doc.Commands, ", "))
	return err
}

func recordHuman(renderer Renderer, value any) {
	if renderer != nil {
		if vr, ok := renderer.(ValueRenderer); ok {
			_ = vr.RenderValue(io.Discard, value)
		}
	}
}

func readiness(ready bool) string {
	if ready {
		return "ready"
	}
	return "missing"
}

func writeVersion(w io.Writer, doc versionDocument) error {
	_, err := fmt.Fprintf(w, "%s %s\nCommit: %s\n", doc.Name, doc.Version, doc.Commit)
	return err
}

func writeExamples(w io.Writer, catalog examplesCatalog) error {
	if _, err := fmt.Fprintln(w, "Examples:"); err != nil {
		return err
	}
	for _, item := range catalog.Examples {
		if _, err := fmt.Fprintf(w, "- %s: %s [%s] (%s)\n", item.ID, item.Purpose, strings.Join(item.Covers, ", "), item.Cost); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "Next: %s\n", catalog.NextStep)
	return err
}

func writeRecipe(w io.Writer, recipe exampleRecipe) error {
	if _, err := fmt.Fprintf(w, "%s\n%s\nCommands: %s\nRequirements: %s\nNetwork calls: %s\nInput: %s\nOutput: %s\nPrivacy: %s\n", recipe.ID, recipe.Purpose, strings.Join(recipe.Covers, ", "), strings.Join(recipe.Requirements, ", "), recipe.Cost, recipe.InputShape, recipe.OutputShape, recipe.Privacy); err != nil {
		return err
	}
	if recipe.Exits != "" {
		if _, err := fmt.Fprintf(w, "Exits: %s\n", recipe.Exits); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "\nShell:\n%s\nNext: %s\n", recipe.Shell, recipe.NextStep); err != nil {
		return err
	}
	return nil
}

func writeModels(w io.Writer, value any) error {
	// Models are intentionally rendered from the stable public fields only.
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Map {
		entry := v.MapIndex(reflect.ValueOf("models"))
		if entry.IsValid() {
			if models, ok := entry.Interface().([]any); ok {
				for _, model := range models {
					if err := writeModel(w, model); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}
	_, err := fmt.Fprintln(w, value)
	return err
}

func writeModel(w io.Writer, value any) error {
	m, ok := value.(map[string]any)
	if !ok {
		_, err := fmt.Fprintln(w, value)
		return err
	}
	_, err := fmt.Fprintf(w, "%s: %s (%s)\n", m["name"], m["description"], m["release_date"])
	return err
}
