package generate

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func project(dir string, watched int) AtlantisProject {
	return AtlantisProject{Dir: dir, Autoplan: AutoplanConfig{WhenModified: make([]string, watched)}}
}

// The project tree has to read well for every repo shape the wrapper serves:
// units at the top level, units under a deep prefix, and a unit that is the
// parent directory of other units.
func TestJoblogProjects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		projects []AtlantisProject
		want     string
	}{
		{
			name:     "flat repo stays a flat list",
			projects: []AtlantisProject{project("humans", 8), project("settings", 5)},
			want: `
    humans    8 paths
    settings  5 paths`,
		},
		{
			name:     "the single-child prefix collapses into one heading",
			projects: []AtlantisProject{project("prod/us-east-1/db", 8), project("prod/us-east-1/web", 9)},
			want: `
    prod/us-east-1/  (2 projects)
      db             8 paths
      web            9 paths`,
		},
		{
			name:     "siblings split the tree where they diverge",
			projects: []AtlantisProject{project("live/qa/db", 8), project("live/stage/db", 8)},
			want: `
    live/       (2 projects)
      qa/db     8 paths
      stage/db  8 paths`,
		},
		{
			name:     "a unit above other units keeps its own row",
			projects: []AtlantisProject{project("iam", 3), project("iam/roles", 4)},
			want: `
    iam/     (2 projects)
      .      3 paths
      roles  4 paths`,
		},
		{
			// --preserve-projects carries over the old output file, and Atlantis
			// allows several projects on one dir; the header counts them all, so
			// the tree has to show them all.
			name:     "projects sharing a dir each keep a row",
			projects: []AtlantisProject{project("module", 1), project("module", 2)},
			want: `
    module  1 path
    module  2 paths`,
		},
		{
			name:     "shared dirs under a heading stay countable",
			projects: []AtlantisProject{project("iam", 3), project("iam", 4), project("iam/roles", 5)},
			want: `
    iam/     (3 projects)
      .      3 paths
      .      4 paths
      roles  5 paths`,
		},
		{
			name:     "the repo root as a unit",
			projects: []AtlantisProject{project(".", 3)},
			want: `
    .  3 paths`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			joblog{&buf}.projects(tc.projects)
			got, want := buf.String(), strings.TrimPrefix(tc.want, "\n")+"\n"
			if got != want {
				t.Errorf("tree renders as\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// A diagnostic is one entry of a list, so its continuation lines indent;
// prose is one block, so it does not.
func TestJoblogFolding(t *testing.T) {
	text := strings.Repeat("word ", 30)
	var entry, prose bytes.Buffer
	joblog{&entry}.wrap(1, "%s", text)
	joblog{&prose}.para(1, "%s", text)

	for name, buf := range map[string]*bytes.Buffer{"wrap": &entry, "para": &prose} {
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if len(line) > wrapWidth {
				t.Errorf("%s produced a %d-column line: %q", name, len(line), line)
			}
		}
	}
	if second := strings.Split(entry.String(), "\n")[1]; !strings.HasPrefix(second, indent(2)) {
		t.Errorf("a list entry must hang its continuation, got %q", second)
	}
	if second := strings.Split(prose.String(), "\n")[1]; strings.HasPrefix(second, indent(2)) {
		t.Errorf("prose must keep one indent, got %q", second)
	}
}

// The phases have to be findable by eye in a few hundred lines, so the banner
// is framed and every section title fills a full-width rule.
func TestJoblogSeparators(t *testing.T) {
	var buf bytes.Buffer
	log := joblog{&buf}
	log.banner("terragrunt-atlantis-gen %s", "v1.2.3")
	log.section("Discovery")
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	rule := strings.Repeat("=", wrapWidth)
	if lines[0] != rule || lines[2] != rule {
		t.Errorf("the banner must sit between two full-width rules:\n%s", buf.String())
	}
	if !strings.Contains(lines[1], "terragrunt-atlantis-gen v1.2.3") {
		t.Errorf("the banner must name the build, got %q", lines[1])
	}
	if lines[3] != "" {
		t.Errorf("a section must open with a blank line, got %q", lines[3])
	}
	if title := lines[4]; len(title) != wrapWidth || !strings.HasPrefix(title, "==== Discovery =") {
		t.Errorf("a section title must fill the rule, got %q", title)
	}
}

// The version the hook ran reaches the log: it is how a repo's atlantis.yaml
// is traced back to a release.
func TestRunLogsGeneratorVersion(t *testing.T) {
	bin := os.Getenv("TERRAGRUNT_BIN")
	if bin == "" {
		bin = "terragrunt"
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "basic_module"))
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	opts := Options{Root: root, TerragruntBin: bin, Version: "v9.9.9", LogWriter: &log}
	if _, err := Run(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "terragrunt-atlantis-gen v9.9.9") {
		t.Errorf("job log does not name the generator version:\n%s", log.String())
	}
}

// The version probe must run where discovery runs: a version-manager shim
// resolves the binary by working directory, so probing in the process's own
// directory can name a different binary than the one that does the work — or
// none at all.
func TestTerragruntVersionRunsInTheRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in binary is a shell script")
	}
	bin := filepath.Join(t.TempDir(), "terragrunt")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"terragrunt version $(pwd)\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := terragruntVersion(bin, root); got != root {
		t.Errorf("the probe ran in %q, want the root %q", got, root)
	}
}

// --preserve-projects carries over entries of the previous output that no unit
// produced. Counting them into "N projects from M units" would state a
// provenance they do not have.
func TestRunLogsPreservedProjectsApart(t *testing.T) {
	bin := os.Getenv("TERRAGRUNT_BIN")
	if bin == "" {
		bin = "terragrunt"
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "basic_module"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "atlantis.yaml")
	previous := `projects:
- autoplan:
    enabled: false
    when_modified:
    - '*.hcl'
  dir: someDir
  name: projectFromPreviousRun
`
	if err := os.WriteFile(out, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	opts := Options{
		Root: root, TerragruntBin: bin, IgnoreParentTerragrunt: true,
		PreserveProjects: true, OutputPath: out, LogWriter: &log,
	}
	if _, err := Run(opts); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2 projects in ", "1 project built from 1 unit", "1 project preserved from the previous output"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, log.String())
		}
	}
	if strings.Contains(log.String(), "2 projects from 1 unit") {
		t.Errorf("a preserved project must not be claimed to come from a unit:\n%s", log.String())
	}
}
