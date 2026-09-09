// Command freeze regenerates the goldens by running a binary over every case.
// Run it from the repo root:
//
//	go run ./tools/freeze -tac /path/to/terragrunt-atlantis-config
//	go run ./tools/freeze -wrapper /path/to/terragrunt-atlantis-gen
//
// -tac writes TAC's output: to testdata/goldens for byte-matching cases and
// to testdata/goldens-tac for diverging ones. -wrapper writes the wrapper's
// output to testdata/goldens for the diverging cases only; everything else
// is TAC's by definition. Regenerating is a deliberate act, never part of a
// normal test run.
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
	wrapper := flag.String("wrapper", "", "path to the wrapper binary; freezes the reviewed goldens of the diverging cases")
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
	for _, dir := range []string{"goldens", "goldens-tac"} {
		if err := os.MkdirAll(filepath.Join(repoRoot, "testdata", dir), 0o755); err != nil {
			fatal(err)
		}
	}

	failed := 0
	for _, c := range goldens.Cases {
		bin, path := *tac, c.TACGoldenPath()
		if *wrapper != "" {
			if c.Diverges == "" {
				continue
			}
			bin, path = *wrapper, c.GoldenPath()
		}
		got, err := c.Replay(bin, repoRoot)
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
