package generate

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// mergeBaseState folds each unit's base-state includes, reads and dependency
// blocks into its head-state Unit. Atlantis matches a pull request's modified
// files — deletions and the old name of a rename included — against
// when_modified generated from the head checkout, where a deleted file no
// longer matches any glob and a config that read it no longer parses (find
// swallows that: exit 0, DEBUG log only). So discovery runs again in a
// detached worktree of the base ref, and a unit watches everything it read
// in either state. Units that exist only in the base get nothing: Atlantis
// cannot plan a directory that is gone.
func mergeBaseState(opts Options, rootAbs string, units map[string]*Unit, parseErrs *parseErrors) error {
	topLevel, err := gitOutput(rootAbs, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("--base-ref needs the root inside a git work tree: %w", err)
	}
	prefix, err := gitOutput(rootAbs, "rev-parse", "--show-prefix")
	if err != nil {
		return err
	}
	commit, err := gitOutput(rootAbs, "rev-parse", "--short", "--verify", "--quiet", opts.BaseRef+"^{commit}")
	if err != nil {
		return fmt.Errorf("--base-ref %q does not resolve to a commit in %s (a branch-strategy checkout needs `git fetch --depth=1 origin <base>` first): %w", opts.BaseRef, topLevel, err)
	}
	start := time.Now()

	worktree, err := os.MkdirTemp("", "terragrunt-atlantis-gen-base-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(worktree)
	// Same reason the root is resolved in Run: terragrunt relativizes
	// against the resolved directory.
	if worktree, err = filepath.EvalSymlinks(worktree); err != nil {
		return err
	}
	if _, err := gitOutput(topLevel, "worktree", "add", "--detach", "--quiet", worktree, opts.BaseRef); err != nil {
		return err
	}
	defer func() {
		if _, err := gitOutput(topLevel, "worktree", "remove", "--force", worktree); err != nil {
			opts.logf("removing base worktree: %v", err)
		}
	}()

	// A root below the top level may be new in the pull request, or have
	// been a file in the base: then every unit is new and there is nothing
	// to merge.
	baseRoot := filepath.Join(worktree, filepath.FromSlash(prefix))
	if info, err := os.Stat(baseRoot); errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		opts.logf("base state %s: %s is not a directory there, every unit is new", commit, filepath.ToSlash(filepath.Clean(prefix)))
		return nil
	} else if err != nil {
		return err
	}

	opts.logf("discovering the base state at %s (%s)", opts.BaseRef, commit)
	baseUnits, suppressed, err := findUnits(opts.TerragruntBin, rootAbs, baseRoot, "--dependencies", "--include", "--reading")
	if err != nil {
		return err
	}
	parseErrs.add(suppressed, baseRoot, "base "+opts.BaseRef)

	slices.SortFunc(baseUnits, func(a, b Unit) int { return strings.Compare(a.Path, b.Path) })
	unitsGained, pathsGained := 0, 0
	var onlyBase []string
	for _, bu := range baseUnits {
		if bu.Type != "unit" {
			continue
		}
		hu, ok := units[bu.Path]
		if !ok {
			onlyBase = append(onlyBase, bu.Path)
			continue
		}
		// Includes and reads share one namespace: find lists an included
		// file under both, and either one puts it in when_modified.
		watched := map[string]bool{}
		for _, p := range hu.Include {
			watched[p] = true
		}
		for _, r := range hu.Reading {
			watched[r] = true
		}
		var added []string
		for _, label := range slices.Sorted(maps.Keys(bu.Include)) {
			p := bu.Include[label]
			if watched[p] {
				continue
			}
			if hu.Include == nil {
				hu.Include = map[string]string{}
			}
			// Only the paths are ever read (sortedIncludePaths); the key just
			// has to be free, and a head label may be anything, "base:x" too.
			key := "base:" + label
			for i := 2; hu.Include[key] != ""; i++ {
				key = fmt.Sprintf("base:%s#%d", label, i)
			}
			hu.Include[key] = p
			watched[p] = true
			added = append(added, p)
		}
		for _, r := range bu.Reading {
			if watched[r] {
				continue
			}
			hu.Reading = append(hu.Reading, r)
			watched[r] = true
			added = append(added, r)
		}
		for _, d := range bu.Dependencies {
			if !slices.Contains(hu.Dependencies, d) {
				hu.Dependencies = append(hu.Dependencies, d)
				added = append(added, d+"/")
			}
		}
		if len(added) == 0 {
			continue
		}
		unitsGained++
		pathsGained += len(added)
		slices.Sort(hu.Reading) // find reports reads sorted; keep that after the union
		slices.Sort(added)
		opts.logf("%s also watches (only in %s): %s", bu.Path, opts.BaseRef, strings.Join(added, ", "))
	}
	for _, p := range onlyBase {
		opts.logf("%s exists only in %s: no project", p, opts.BaseRef)
	}
	opts.logf("base state %s: %d units watch %d more paths, %d units gone, in %s", commit, unitsGained, pathsGained, len(onlyBase), time.Since(start).Round(time.Millisecond))
	return nil
}

// gitOutput runs git in dir and returns its trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s in %s: %w\n%s", strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
