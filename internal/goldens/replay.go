package goldens

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const fixturesDir = "testdata/fixtures/"

// Replay runs bin's generate command for the case from the repo root and
// returns the produced output file.
func (c Case) Replay(bin, repoRoot string) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "goldens-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	out := filepath.Join(tmp, "atlantis.yaml")
	if c.PreSeed != "" {
		if err := os.WriteFile(out, []byte(c.PreSeed), 0o644); err != nil {
			return nil, err
		}
	}

	// Plain concatenation: a trailing slash in Fixture must reach --root.
	root := fixturesDir + c.Fixture
	if c.AbsRoot {
		root, err = filepath.Abs(filepath.Join(repoRoot, root))
		if err != nil {
			return nil, err
		}
	}
	if c.History != nil {
		root = filepath.Join(tmp, "repo")
		if err := c.History.build(filepath.Join(repoRoot, fixturesDir, c.Fixture), root); err != nil {
			return nil, err
		}
	}
	args := append([]string{"generate", "--output", out, "--root", root}, c.Flags...)
	// A version-manager shim on the PATH resolves by working directory and
	// knows nothing about the temporary repositories; TERRAGRUNT_BIN names
	// the real binary.
	if bin := os.Getenv("TERRAGRUNT_BIN"); bin != "" {
		args = append(args, "--terragrunt-bin", bin)
	}
	if len(c.Filter) > 0 {
		globs := make([]string, len(c.Filter))
		for i, f := range c.Filter {
			globs[i] = fixturesDir + c.Fixture + "/" + f
		}
		args = append(args, "--filter", strings.Join(globs, ","))
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s %v: %w\nstderr: %s", bin, args, err, stderr.String())
	}
	return os.ReadFile(out)
}

// build copies the fixture to dir and commits it twice: the fixture on
// `base`, the pull request on `head`. Git runs without the user's config so
// no hook or signing setup interferes.
func (h *History) build(fixture, dir string) error {
	if err := copyTree(fixture, dir); err != nil {
		return err
	}
	hooks := filepath.Join(dir, "..", "nohooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		return err
	}
	git := func(args ...string) error {
		cmd := exec.Command("git", append([]string{
			"-c", "user.name=goldens", "-c", "user.email=goldens@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "init.defaultBranch=base", "-c", "core.hooksPath=" + hooks,
		}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w\n%s", args, err, out)
		}
		return nil
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"commit", "-q", "-m", "base"}, {"checkout", "-q", "-b", "head"}} {
		if err := git(args...); err != nil {
			return err
		}
	}
	for _, p := range h.Delete {
		if err := os.Remove(filepath.Join(dir, p)); err != nil {
			return err
		}
		// Git tracks no directories: a checkout of the head has none left
		// behind where the last file went.
		for parent := filepath.Dir(filepath.Join(dir, p)); parent != dir; parent = filepath.Dir(parent) {
			if os.Remove(parent) != nil {
				break
			}
		}
	}
	for from, to := range h.Rename {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, to)), 0o755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(dir, from), filepath.Join(dir, to)); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "pull request"}} {
		if err := git(args...); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// GoldenPath returns the case's golden file, relative to the repo root.
func (c Case) GoldenPath() string {
	return filepath.Join("testdata", "goldens", c.Name+".yaml")
}
