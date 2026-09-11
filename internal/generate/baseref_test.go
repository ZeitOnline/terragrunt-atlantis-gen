package generate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitIn runs git in repo without the user's configuration, so no hook or
// signing setup interferes.
func gitIn(t *testing.T, repo string) func(args ...string) {
	t.Helper()
	return func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=test", "-c", "user.email=test@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=base",
		}, args...)...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func writeIn(t *testing.T, repo string) func(rel, content string) {
	t.Helper()
	return func(rel, content string) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A --root that the pull request introduced is not a directory in the base:
// the base pass has nothing to merge and must not fail the run.
func TestMergeBaseStateRootMissingInBase(t *testing.T) {
	repo := t.TempDir()
	git, writeFile := gitIn(t, repo), writeIn(t, repo)
	write := func(rel string) { writeFile(rel, "x") }

	git("init", "-q")
	write("existing/terragrunt.hcl")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "head")
	write("added/unit/terragrunt.hcl")
	git("add", "-A")
	git("commit", "-q", "-m", "head")

	root, err := filepath.EvalSymlinks(filepath.Join(repo, "added"))
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	units := map[string]*Unit{"unit": {Type: "unit", Path: "unit", Reading: []string{"x.hcl"}}}
	eligible := map[string]bool{"unit": true}
	if err := mergeBaseState(Options{BaseRef: "base", LogWriter: &log}, root, units, eligible, &parseErrors{}); err != nil {
		t.Fatalf("base pass must skip a root missing in the base, got: %v", err)
	}
	if got := units["unit"].Reading; len(got) != 1 || got[0] != "x.hcl" {
		t.Errorf("head unit changed: %v", got)
	}
	if !strings.Contains(log.String(), "added is not a directory there") {
		t.Errorf("log does not say why the base pass was skipped:\n%s", log.String())
	}
	// Creating the worktree is the slow part and happens before the skip, so
	// the section reports its time on this path too.
	if !strings.Contains(log.String(), "took ") {
		t.Errorf("the skipped base pass omits its elapsed time:\n%s", log.String())
	}
	out, _ := exec.Command("git", "-C", repo, "worktree", "list").Output()
	if lines := strings.Count(strings.TrimSpace(string(out)), "\n"); lines != 0 {
		t.Errorf("worktree left behind:\n%s", out)
	}
}

// A dependency block the run ignores reaches no when_modified entry, so the
// base pass must not report it as a gained path: the section counts what a
// unit watches, not what its config declares. The union keeps it either way.
func TestMergeBaseStateIgnoredDependencyBlocks(t *testing.T) {
	bin := os.Getenv("TERRAGRUNT_BIN")
	if bin == "" {
		bin = "terragrunt"
	}
	repo := t.TempDir()
	git, writeFile := gitIn(t, repo), writeIn(t, repo)
	writeFile("root.hcl", "")
	writeFile("dep/terragrunt.hcl", `include "root" {
  path = find_in_parent_folders("root.hcl")
}
`)
	writeFile("consumer/terragrunt.hcl", `include "root" {
  path = find_in_parent_folders("root.hcl")
}

dependency "dep" {
  config_path = "../dep"
}
`)
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "base")

	root, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ignore   bool
		reported bool
	}{
		{ignore: false, reported: true},
		{ignore: true, reported: false},
	} {
		// The head state matches the base but for the dependency block.
		units := map[string]*Unit{"consumer": {
			Type:    "unit",
			Path:    "consumer",
			Include: map[string]string{"root": "root.hcl"},
			Reading: []string{"root.hcl"},
		}}
		var log bytes.Buffer
		opts := Options{BaseRef: "base", TerragruntBin: bin, IgnoreDependencyBlocks: tc.ignore, LogWriter: &log}
		if err := mergeBaseState(opts, root, units, map[string]bool{"consumer": true}, &parseErrors{}); err != nil {
			t.Fatalf("--ignore-dependency-blocks=%v: %v", tc.ignore, err)
		}
		if got := strings.Contains(log.String(), "+ dep/"); got != tc.reported {
			t.Errorf("--ignore-dependency-blocks=%v reports dep/ as watched = %v, want %v:\n%s",
				tc.ignore, got, tc.reported, log.String())
		}
		if !slices.Contains(units["consumer"].Dependencies, "dep") {
			t.Errorf("--ignore-dependency-blocks=%v dropped the base dependency from the union: %v",
				tc.ignore, units["consumer"].Dependencies)
		}
	}
}

// A unit that --filter or an exclude block drops builds no project, so a path
// it gained from the base reaches no when_modified entry: the section must not
// count it. The merge still runs, so nothing about the state is lost.
func TestMergeBaseStateScopedToEligibleUnits(t *testing.T) {
	bin := os.Getenv("TERRAGRUNT_BIN")
	if bin == "" {
		bin = "terragrunt"
	}
	repo := t.TempDir()
	git, writeFile := gitIn(t, repo), writeIn(t, repo)
	writeFile("root.hcl", "")
	for _, unit := range []string{"planned", "dropped"} {
		writeFile("settings/"+unit+".hcl", "locals {}\n")
		writeFile(unit+"/terragrunt.hcl", `include "root" {
  path = find_in_parent_folders("root.hcl")
}

locals {
  s = read_terragrunt_config("${get_parent_terragrunt_dir()}/settings/`+unit+`.hcl")
}
`)
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "base")

	root, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	// Both units read a settings file in the base and neither does in the head.
	units := map[string]*Unit{}
	for _, unit := range []string{"planned", "dropped"} {
		units[unit] = &Unit{
			Type:    "unit",
			Path:    unit,
			Include: map[string]string{"root": "root.hcl"},
			Reading: []string{"root.hcl"},
		}
	}
	var log bytes.Buffer
	opts := Options{BaseRef: "base", TerragruntBin: bin, LogWriter: &log}
	if err := mergeBaseState(opts, root, units, map[string]bool{"planned": true}, &parseErrors{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "settings/planned.hcl") {
		t.Errorf("the planned unit's gained path is missing:\n%s", log.String())
	}
	if strings.Contains(log.String(), "settings/dropped.hcl") {
		t.Errorf("a unit with no project must not be reported as watching more:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "1 path across 1 unit") {
		t.Errorf("the counts must cover the reported unit only:\n%s", log.String())
	}
	// Merged all the same: the Unit stays a faithful union of both states.
	for _, unit := range []string{"planned", "dropped"} {
		if !slices.Contains(units[unit].Reading, "settings/"+unit+".hcl") {
			t.Errorf("%s did not get the base read merged: %v", unit, units[unit].Reading)
		}
	}
}
