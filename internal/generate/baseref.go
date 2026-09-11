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
//
// Every unit is merged, but only the units in eligible — the ones this run may
// build a project for — are reported: a path gained by a unit that --filter or
// an exclude block drops reaches no when_modified entry.
func mergeBaseState(opts Options, rootAbs string, units map[string]*Unit, eligible map[string]bool, parseErrs *parseErrors) error {
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
	log := joblog{opts.LogWriter}
	log.section("Base state  %s @ %s", opts.BaseRef, commit)

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
			log.line(1, "warning: removing the base worktree failed: %v", err)
		}
	}()

	// A root below the top level may be new in the pull request, or have
	// been a file in the base: then every unit is new and there is nothing
	// to merge.
	baseRoot := filepath.Join(worktree, filepath.FromSlash(prefix))
	if info, err := os.Stat(baseRoot); errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		log.line(1, "%s is not a directory there: every unit is new, nothing to merge", filepath.ToSlash(filepath.Clean(prefix)))
		log.line(1, "took %s", since(start))
		return nil
	} else if err != nil {
		return err
	}

	baseUnits, suppressed, err := findUnits(opts.TerragruntBin, rootAbs, baseRoot, "--dependencies", "--include", "--reading")
	if err != nil {
		return err
	}
	parseErrs.add(suppressed, baseRoot, "base "+opts.BaseRef)

	slices.SortFunc(baseUnits, func(a, b Unit) int { return strings.Compare(a.Path, b.Path) })
	unitsGained, pathsGained := 0, 0
	var onlyBase []string
	var gained []row
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
		// The union stays faithful to both states, but a dependency block the
		// run ignores reaches no when_modified entry (see builder.dependencies),
		// and this section counts what a unit watches, not what it declares.
		for _, d := range bu.Dependencies {
			if slices.Contains(hu.Dependencies, d) {
				continue
			}
			hu.Dependencies = append(hu.Dependencies, d)
			if !opts.IgnoreDependencyBlocks {
				added = append(added, d+"/")
			}
		}
		if len(added) == 0 {
			continue
		}
		slices.Sort(hu.Reading) // find reports reads sorted; keep that after the union
		if !eligible[bu.Path] {
			continue
		}
		unitsGained++
		pathsGained += len(added)
		slices.Sort(added)
		// One row per path, the unit named on the first of them: a unit that
		// gained a dozen paths stays one block instead of one wrapped line.
		for i, p := range added {
			label := bu.Path
			if i > 0 {
				label = ""
			}
			gained = append(gained, row{2, label, "+ " + p})
		}
	}
	if len(gained) > 0 {
		log.line(1, "%s across %s watched only because of the base:", plural(pathsGained, "path"), plural(unitsGained, "unit"))
		log.table(gained)
	}
	if len(onlyBase) > 0 {
		log.line(1, "%s only in the base, so no project:", plural(len(onlyBase), "unit"))
		for _, p := range onlyBase {
			log.line(2, "%s", p)
		}
	}
	if len(gained) == 0 && len(onlyBase) == 0 {
		log.line(1, "nothing to merge: the base watches nothing the head does not")
	}
	log.line(1, "took %s", since(start))
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
