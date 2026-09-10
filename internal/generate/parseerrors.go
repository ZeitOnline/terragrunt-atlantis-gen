package generate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// parseErrors collects what discovery suppressed across the find runs, one
// line per distinct error, paths relative to the tree they came from.
type parseErrors struct {
	seen  map[string]bool
	lines []string
}

// add records the suppressed messages of one find run over root; label
// names the run when it is not the head ("base origin/main").
func (p *parseErrors) add(msgs []string, root, label string) {
	if p.seen == nil {
		p.seen = map[string]bool{}
	}
	for _, msg := range msgs {
		msg = strings.TrimPrefix(msg, "Suppressed parsing errors ")
		msg = strings.TrimPrefix(msg, "Suppressed parse error for ")
		msg = strings.ReplaceAll(msg, root+string(filepath.Separator), "")
		msg = strings.Join(strings.Fields(msg), " ") // diagnostics span lines
		if label != "" {
			msg = fmt.Sprintf("%s (in %s)", msg, label)
		}
		if !p.seen[msg] {
			p.seen[msg] = true
			p.lines = append(p.lines, msg)
		}
	}
}

// report writes every collected error to the job log and, unless the run
// was told to carry on, turns them into its error.
func (p *parseErrors) report(opts Options) error {
	if len(p.lines) == 0 {
		return nil
	}
	for _, l := range p.lines {
		opts.logf("terragrunt suppressed a parse error: %s", l)
	}
	opts.logf("%d parse error(s) suppressed by terragrunt find; the affected units watch fewer paths than their configs declare", len(p.lines))
	if opts.FailOnParseErrors {
		return fmt.Errorf("%d config(s) do not parse (--fail-on-parse-errors):\n  %s", len(p.lines), strings.Join(p.lines, "\n  "))
	}
	return nil
}
