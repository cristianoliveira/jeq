package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/contract"
)

func writeHome(w io.Writer, doc homeDocument) error {
	_, err := fmt.Fprintf(w, "gev\n%s\nCredentials: %s\nDefault model: %s (%s)\nCommands: %s\nNext: gev examples\n", doc.Purpose, readiness(doc.CredentialReady), doc.DefaultModel, doc.DefaultModelSource, strings.Join(doc.Commands, ", "))
	return err
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

func writeModels(w io.Writer, models contract.Models) error {
	for _, model := range models.Models {
		if _, err := fmt.Fprintf(w, "%s: %s (%s)\n", model.Name, model.Description, model.ReleaseDate); err != nil {
			return err
		}
	}
	return nil
}
