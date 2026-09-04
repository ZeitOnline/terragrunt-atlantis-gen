// Command freeze runs the frozen TAC binary over every golden case and
// writes the outputs to testdata/goldens/. Run it from the repo root:
//
//	go run ./tools/freeze -tac /path/to/terragrunt-atlantis-config
//
// The goldens define "same as TAC" for the wrapper; regenerating them is a
// deliberate act, never part of a normal test run.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZeitOnline/terragrunt-atlantis-gen/internal/goldens"
)

func main() {
	tac := flag.String("tac", "", "path to the terragrunt-atlantis-config binary to freeze")
	flag.Parse()
	if *tac == "" {
		fmt.Fprintln(os.Stderr, "usage: freeze -tac <terragrunt-atlantis-config binary>")
		os.Exit(2)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "testdata", "fixtures")); err != nil {
		fatal(fmt.Errorf("run from the repo root: %w", err))
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "testdata", "goldens"), 0o755); err != nil {
		fatal(err)
	}

	failed := 0
	for _, c := range goldens.Cases {
		got, err := c.Replay(*tac, repoRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", c.Name, err)
			failed++
			continue
		}
		path := filepath.Join(repoRoot, c.GoldenPath())
		if err := os.WriteFile(path, got, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("%7d  %s\n", len(got), c.GoldenPath())
	}
	if failed > 0 {
		fatal(fmt.Errorf("%d case(s) failed", failed))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "freeze:", err)
	os.Exit(1)
}
