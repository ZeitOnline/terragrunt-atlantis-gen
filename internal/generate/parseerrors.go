package generate

import (
	"cmp"
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
	var reported []parseError
	for _, e := range p.items {
		if keep == nil || e.concerns(units, keep) {
			reported = append(reported, e)
		}
	}
	if len(reported) == 0 {
		return nil
	}
	// find reports them in discovery order, which differs between runs of the
	// same tree; by path the same repo always reads the same way.
	slices.SortFunc(reported, func(a, b parseError) int {
		return cmp.Or(strings.Compare(a.path, b.path), strings.Compare(a.text, b.text))
	})
	log := joblog{opts.LogWriter}
	log.section("Parse errors  %d suppressed by terragrunt find", len(reported))
	for _, e := range reported {
		log.wrap(1, "%s", e.text)
	}
	log.write("")
	if !opts.FailOnParseErrors {
		log.para(1, "These configs do not parse, so the units that read them watch fewer "+
			"paths than they declare and a change to those paths triggers no plan.")
		return nil
	}
	log.para(1, "These configs do not parse, so the units that read them watch fewer "+
		"paths than they declare. Pass --fail-on-parse-errors=false to generate anyway.")
	// The diagnostics are already in the section above; the error names the
	// configs to fix, and falls back to the whole message where one names none.
	// Several diagnostics in one file are one config to fix.
	var blamed []string
	seen := map[string]bool{}
	for _, e := range reported {
		name := e.path
		if name == "" {
			name = e.text
		}
		if !seen[name] {
			seen[name] = true
			blamed = append(blamed, name)
		}
	}
	return fmt.Errorf("%s did not parse (--fail-on-parse-errors):\n  %s",
		plural(len(blamed), "config"), strings.Join(blamed, "\n  "))
}
