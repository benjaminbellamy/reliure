package cli

import (
	"context"
	"fmt"
)

// DiffCmd is a v0.2 stub. Will compare two snapshots.
type DiffCmd struct {
	A string `arg:"" help:"First snapshot." type:"existingfile"`
	B string `arg:"" help:"Second snapshot." type:"existingfile"`
}

// Run prints a placeholder message until v0.2.
func (c *DiffCmd) Run(ctx context.Context) error {
	fmt.Println("`reliure diff` is not implemented yet (v0.2).")
	return nil
}
