package generate

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// parseError is one message discovery suppressed: the text for the job log
// and the file or directory it names, relative to its tree, empty when the
// message names none.
type parseError struct {
	path string
	text string
}

// parseErrors collects what discovery suppressed across the find runs, one
// entry per distinct error.
type parseErrors struct {
	seen  map[string]bool
	items []parseError
}

// add records the suppressed messages of one find run over root; label
// names the run when it is not the head ("base origin/main").
func (p *parseErrors) add(msgs []string, root, label string) {
	if p.seen == nil {
		p.seen = map[string]bool{}
	}
	// "Suppressed parse error for <dir>" names a directory; for a unit at
	// the root that is the bare root, so it becomes "." rather than staying
	// absolute. The boundary keeps a sibling like <root>-old intact.
	bareRoot := regexp.MustCompile(regexp.QuoteMeta(root) + `([\s:"']|$)`)
	for _, msg := range msgs {
		msg = suppressedParse.ReplaceAllString(msg, "")
		// Every wording puts the file or directory first, up to the colon
		// before the position or the diagnostic.
		var path string
		if i := strings.IndexByte(msg, ':'); i > 0 {
			if rel, err := filepath.Rel(root, msg[:i]); err == nil && !strings.HasPrefix(rel, "..") {
				path = filepath.ToSlash(rel)
			}
		}
		msg = strings.ReplaceAll(msg, root+string(filepath.Separator), "")
		msg = bareRoot.ReplaceAllString(msg, ".$1")
		msg = strings.Join(strings.Fields(msg), " ") // diagnostics span lines
		if label != "" {
			msg = fmt.Sprintf("%s (in %s)", msg, label)
		}
		if !p.seen[msg] {
			p.seen[msg] = true
			p.items = append(p.items, parseError{path: path, text: msg})
		}
	}
}

// concerns reports whether the error can change what a kept unit watches:
// it sits in the unit's directory or in a file the unit includes or reads.
// An error no unit owns counts for all of them, so a filter never hides it.
func (e parseError) concerns(units map[string]*Unit, keep map[string]bool) bool {
	if e.path == "" {
		return true
	}
	owned := false
	for unitPath, u := range units {
		inDir := unitPath == "." || e.path == unitPath || strings.HasPrefix(e.path, unitPath+"/")
		if !inDir && !u.touches(e.path) {
			continue
		}
		owned = true
		if keep[unitPath] {
			return true
		}
	}
	return !owned
}

// touches reports whether the unit includes or reads the file at p.
func (u *Unit) touches(p string) bool {
	for _, inc := range u.Include {
		if inc == p {
			return true
		}
	}
	return slices.Contains(u.Reading, p)
}

// report writes the errors that concern a kept unit to the job log and,
// unless the run was told to carry on, turns them into its error. keep nil
// means every unit is kept.
func (p *parseErrors) report(opts Options, units map[string]*Unit, keep map[string]bool) error {
	var lines []string
	for _, e := range p.items {
		if keep == nil || e.concerns(units, keep) {
			lines = append(lines, e.text)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	for _, l := range lines {
		opts.logf("terragrunt suppressed a parse error: %s", l)
	}
	opts.logf("%d parse error(s) suppressed by terragrunt find; the affected units watch fewer paths than their configs declare", len(lines))
	if opts.FailOnParseErrors {
		return fmt.Errorf("%d config(s) do not parse (--fail-on-parse-errors):\n  %s", len(lines), strings.Join(lines, "\n  "))
	}
	return nil
}
