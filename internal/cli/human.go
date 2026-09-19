package cli

import (
	"fmt"
	"io"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
)

func writeVersion(w io.Writer, doc versionDocument) error {
	_, err := fmt.Fprintf(w, "%s %s\nCommit: %s\n", doc.Name, doc.Version, doc.Commit)
	return err
}

func writeModels(w io.Writer, models contract.Models) error {
	for _, model := range models.Models {
		if _, err := fmt.Fprintf(w, "%s: %s (%s)\n", model.Name, model.Description, model.ReleaseDate); err != nil {
			return err
		}
	}
	return nil
}
