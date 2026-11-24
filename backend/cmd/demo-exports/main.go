// Command demo-exports writes the synthetic demo exports to a directory: the XTB xlsx files and the Kraken CSV file.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jrojowski/profit-tracker/backend/internal/demo"
	"github.com/jrojowski/profit-tracker/backend/internal/kraken/krakentest"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

func main() {
	out := flag.String("out", "", "directory for the demo export files")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "demo-exports:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	if out == "" {
		return errors.New("flag -out is required")
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	for _, a := range demo.Accounts() {
		data, err := xtbtest.BuildFile(a.Export)
		if err != nil {
			return fmt.Errorf("build export of %s: %w", a.Name, err)
		}
		path := filepath.Join(out, a.Export.FileName())
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Println(path)
	}
	kraken := demo.Kraken()
	data, err := krakentest.BuildFile(kraken)
	if err != nil {
		return fmt.Errorf("build kraken export: %w", err)
	}
	path := filepath.Join(out, kraken.Name())
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Println(path)

	tiers, err := json.MarshalIndent(demo.Tiers(), "", "  ")
	if err != nil {
		return fmt.Errorf("build tiers: %w", err)
	}
	tiersPath := filepath.Join(out, "tiers.json")
	if err := os.WriteFile(tiersPath, tiers, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tiersPath, err)
	}
	fmt.Println(tiersPath)
	return nil
}
