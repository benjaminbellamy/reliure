package cli

import (
	"context"
	"fmt"
)

// ListCmd is a v0.2 stub. Will print a tabular summary of a snapshot.
type ListCmd struct {
	Snapshot string `arg:"" help:"Path to the snapshot YAML." type:"existingfile"`
}

// Run prints a placeholder message until v0.2.
func (c *ListCmd) Run(ctx context.Context) error {
	fmt.Println("`reliure list` is not implemented yet (v0.2).")
	return nil
}
