package generate

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// joblog formats the Atlantis job log. A run is read in a browser, after the
// fact, by someone asking why a project watches what it watches — so the
// output is a sequence of "==>" sections, each carrying its own counts, with
// the detail indented beneath. A nil writer silences everything.
type joblog struct{ w io.Writer }

// wrapWidth is where prose and diagnostics fold, and how wide the separators
// are drawn. The job view is not a terminal and reports no width; 92 columns
// fit its default layout.
const wrapWidth = 92

// valueColumn caps how far the aligned tables push their value column, so a
// single deep path does not indent every other row off the screen.
const valueColumn = 56

func (l joblog) write(s string) {
	if l.w != nil {
		fmt.Fprintln(l.w, s)
	}
}

// indent returns the leading whitespace of a body line at depth (1 is
// directly under the section header, whose "==> " is four columns wide).
func indent(depth int) string { return strings.Repeat("  ", depth+1) }

// banner opens the log with the build that produced it, framed so the start
// of a run is unmistakable where Atlantis concatenates the hook output.
func (l joblog) banner(format string, args ...any) {
	rule := strings.Repeat("=", wrapWidth)
	l.write(rule)
	l.write("  " + fmt.Sprintf(format, args...))
	l.write(rule)
}

// section starts a phase, its title set in a full-width rule: in a log of a
// few hundred lines the phases have to be findable by eye alone.
func (l joblog) section(format string, args ...any) {
	title := "==== " + fmt.Sprintf(format, args...) + " "
	if len(title) < wrapWidth {
		title += strings.Repeat("=", wrapWidth-len(title))
	}
	l.write("")
	l.write(title)
}

// line writes one body line at depth.
func (l joblog) line(depth int, format string, args ...any) {
	l.write(indent(depth) + fmt.Sprintf(format, args...))
}

// fold writes text at wrapWidth, the first line behind first, every further
// line behind cont. A word longer than the width — a path, usually —
// overflows rather than breaking.
func (l joblog) fold(first, cont, text string) {
	if l.w == nil {
		return
	}
	prefix, line := first, ""
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = prefix + word
		case len(line)+1+len(word) <= wrapWidth:
			line += " " + word
		default:
			l.write(line)
			prefix = cont
			line = prefix + word
		}
	}
	if line != "" {
		l.write(line)
	}
}

// wrap writes one list entry, its continuation lines indented one level
// deeper so the entries stay apart. Terragrunt's diagnostics are single
// paragraphs of a few hundred characters.
func (l joblog) wrap(depth int, format string, args ...any) {
	l.fold(indent(depth), indent(depth+1), fmt.Sprintf(format, args...))
}

// para writes prose as one block, every line at the same depth.
func (l joblog) para(depth int, format string, args ...any) {
	l.fold(indent(depth), indent(depth), fmt.Sprintf(format, args...))
}

// fields writes name/value pairs with the values in one column, a value too
// long for the line continuing in that same column.
func (l joblog) fields(pairs [][2]string) {
	width := 0
	for _, p := range pairs {
		if len(p[0]) > width {
			width = len(p[0])
		}
	}
	for _, p := range pairs {
		first := indent(1) + fmt.Sprintf("%-*s  ", width, p[0])
		l.fold(first, strings.Repeat(" ", len(first)), p[1])
	}
}

// row is one line of an aligned table: a label at depth, and a value all
// rows of the table share a column for. An empty label continues the row
// above, which is how a unit with several entries reads as one block.
type row struct {
	depth int
	label string
	value string
}

func (l joblog) table(rows []row) {
	if l.w == nil {
		return
	}
	width := 0
	for _, r := range rows {
		if w := len(indent(r.depth)) + len(r.label); w > width && w <= valueColumn {
			width = w
		}
	}
	for _, r := range rows {
		lead := indent(r.depth) + r.label
		pad := width - len(lead)
		if pad < 0 {
			pad = 0
		}
		l.write(lead + strings.Repeat(" ", pad+2) + r.value)
	}
}

// projects writes the project list as a path tree: siblings share a heading
// that carries their count, and a chain of directories holding nothing else
// collapses into one line. A repo with its units at the top level renders as
// a flat list, a deeply nested one as a tree — neither needs the layout to
// know the repo's shape.
func (l joblog) projects(projects []AtlantisProject) {
	if l.w == nil {
		return
	}
	root := newPathNode()
	for i := range projects {
		n := root
		for _, seg := range strings.Split(projects[i].Dir, "/") {
			child, ok := n.children[seg]
			if !ok {
				child = newPathNode()
				n.children[seg] = child
			}
			n = child
		}
		n.watched = len(projects[i].Autoplan.WhenModified)
	}
	var rows []row
	root.appendRows(&rows, 1)
	l.table(rows)
}

// pathNode is one segment of the project tree; watched is -1 where no
// project sits on the node itself.
type pathNode struct {
	children map[string]*pathNode
	watched  int
}

func newPathNode() *pathNode { return &pathNode{children: map[string]*pathNode{}, watched: -1} }

func (n *pathNode) leaves() int {
	count := 0
	if n.watched >= 0 {
		count = 1
	}
	for _, c := range n.children {
		count += c.leaves()
	}
	return count
}

func (n *pathNode) appendRows(rows *[]row, depth int) {
	names := make([]string, 0, len(n.children))
	for name := range n.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		child, label := n.children[name], name
		for child.watched < 0 && len(child.children) == 1 {
			for seg, only := range child.children {
				label, child = label+"/"+seg, only
			}
		}
		if len(child.children) == 0 {
			*rows = append(*rows, row{depth, label, plural(child.watched, "path")})
			continue
		}
		*rows = append(*rows, row{depth, label + "/", fmt.Sprintf("(%s)", plural(child.leaves(), "project"))})
		if child.watched >= 0 {
			*rows = append(*rows, row{depth + 1, ".", plural(child.watched, "path")})
		}
		child.appendRows(rows, depth+1)
	}
}

// since renders a phase's elapsed time; sub-millisecond precision would only
// make two runs of the same repo look different.
func since(start time.Time) string { return time.Since(start).Round(time.Millisecond).String() }

// plural renders a count with its noun, "1 unit" against "2 units".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// settings lists the behaviour the run was configured with, so a job log
// answers on its own why the output looks the way it does. Booleans appear
// only when enabled, in their flag spelling.
func (o Options) settings() string {
	var s []string
	for _, f := range []struct {
		name string
		on   bool
	}{
		{"autoplan", o.AutoPlan},
		{"automerge", o.AutoMerge},
		{"parallel", o.Parallel},
		{"create-workspace", o.CreateWorkspace},
		{"create-project-name", o.CreateProjectName},
		{"execution-order-groups", o.ExecutionOrderGroups},
		{"depends-on", o.DependsOn},
		{"cascade-dependencies", o.CascadeDependencies},
		{"ignore-parent-terragrunt", o.IgnoreParentTerragrunt},
		{"ignore-dependency-blocks", o.IgnoreDependencyBlocks},
		{"preserve-workflows", o.PreserveWorkflows},
		{"preserve-projects", o.PreserveProjects},
		{"fail-on-parse-errors", o.FailOnParseErrors},
	} {
		if f.on {
			s = append(s, f.name)
		}
	}
	if o.DefaultWorkflow != "" {
		s = append(s, "workflow="+o.DefaultWorkflow)
	}
	if o.DefaultTerraformVersion != "" {
		s = append(s, "terraform-version="+o.DefaultTerraformVersion)
	}
	if len(o.DefaultApplyRequirements) > 0 {
		s = append(s, "apply-requirements="+strings.Join(o.DefaultApplyRequirements, "+"))
	}
	if len(s) == 0 {
		return "(all defaults off)"
	}
	return strings.Join(s, ", ")
}
