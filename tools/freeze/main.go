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
	wrapper := flag.String("wrapper", "", "path to the wrapper binary; freezes the reviewed wrapper goldens for diverging cases instead")
	flag.Parse()
	if (*tac == "") == (*wrapper == "") {
		fmt.Fprintln(os.Stderr, "usage: freeze -tac <tac binary> | -wrapper <wrapper binary>")
		os.Exit(2)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "testdata", "fixtures")); err != nil {
		fatal(fmt.Errorf("run from the repo root: %w", err))
	}

	// TAC goldens replay on the pristine fixtures; wrapper goldens on the
	// migrated tree, like the parity suite.
	bin, replayRoot, goldensDir := *tac, repoRoot, "goldens"
	if *wrapper != "" {
		bin = *wrapper
		replayRoot = filepath.Join(repoRoot, "testdata", "migrated")
		goldensDir = "goldens-wrapper"
		if err := os.MkdirAll(filepath.Join(replayRoot, "internal", "goldens"), 0o755); err != nil {
			fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "testdata", goldensDir), 0o755); err != nil {
		fatal(err)
	}

	failed := 0
	for _, c := range goldens.Cases {
		path := c.GoldenPath()
		if *wrapper != "" {
			if c.Diverges == "" {
				continue
			}
			path = c.ParityGoldenPath()
		}
		got, err := c.Replay(bin, replayRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", c.Name, err)
			failed++
			continue
		}
		if err := os.WriteFile(filepath.Join(repoRoot, path), got, 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("%7d  %s\n", len(got), path)
	}
	if failed > 0 {
		fatal(fmt.Errorf("%d case(s) failed", failed))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "freeze:", err)
	os.Exit(1)
}
