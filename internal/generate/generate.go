// Package generate produces the atlantis.yaml that terragrunt-atlantis-config
// (TAC) would produce, from `terragrunt find --json` instead of parsed HCL.
// The output logic is ported from TAC v2.25.1 cmd/generate.go so the result
// stays byte-compatible; the goldens in testdata/goldens are the contract.
package generate

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
)

type Options struct {
	Root          string
	TerragruntBin string

	AutoPlan               bool
	AutoMerge              bool
	Parallel               bool
	IgnoreParentTerragrunt bool
	IgnoreDependencyBlocks bool
	CascadeDependencies    bool
	CreateWorkspace        bool
	CreateProjectName      bool
	PreserveWorkflows      bool
	PreserveProjects       bool
	ExecutionOrderGroups   bool
	DependsOn              bool

	DefaultWorkflow          string
	DefaultTerraformVersion  string
	DefaultApplyRequirements []string
	FilterPaths              []string
	OutputPath               string

	// BaseRef is the pull request's base as a git ref; empty skips the base
	// discovery (see mergeBaseState).
	BaseRef string

	// FailOnParseErrors turns the parse errors discovery suppressed into a
	// failed run, TAC's behaviour and the CLI default; false only logs them.
	FailOnParseErrors bool

	// Version names the build in the job log's first line.
	Version string

	// LogWriter receives human-readable progress (the Atlantis job log);
	// nil silences it.
	LogWriter io.Writer
}

// The output structs are TAC's, marshalled with the same library (ghodss)
// via the same JSON tags — byte compatibility by construction.
type AtlantisConfig struct {
	Version       int               `json:"version"`
	AutoMerge     bool              `json:"automerge"`
	ParallelPlan  bool              `json:"parallel_plan"`
	ParallelApply bool              `json:"parallel_apply"`
	Projects      []AtlantisProject `json:"projects,omitempty"`
	Workflows     interface{}       `json:"workflows,omitempty"`
}

type AtlantisProject struct {
	Dir                 string         `json:"dir"`
	Workflow            string         `json:"workflow,omitempty"`
	Workspace           string         `json:"workspace,omitempty"`
	Name                string         `json:"name,omitempty"`
	Autoplan            AutoplanConfig `json:"autoplan"`
	TerraformVersion    string         `json:"terraform_version,omitempty"`
	ApplyRequirements   *[]string      `json:"apply_requirements,omitempty"`
	ExecutionOrderGroup *int           `json:"execution_order_group,omitempty"`
	DependsOn           []string       `json:"depends_on,omitempty"`
}

type AutoplanConfig struct {
	WhenModified []string `json:"when_modified"`
	Enabled      bool     `json:"enabled"`
}

var projectNameRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

var codeFileSuffixes = []string{".tf", ".tf.json", ".tofu", ".tofu.json"}

func isCodeFile(p string) bool {
	for _, s := range codeFileSuffixes {
		if strings.HasSuffix(p, s) {
			return true
		}
	}
	return false
}

type builder struct {
	opts      Options
	gitRoot   string // absolute, with trailing separator (TAC convention)
	rootAbs   string // absolute, without trailing separator
	units     map[string]*Unit
	hasSource map[string]bool
	memo      map[string][]string
	building  map[string]bool
}

// Run generates the config and returns its YAML bytes; the caller decides
// where they go.
func Run(opts Options) ([]byte, error) {
	if opts.TerragruntBin == "" {
		opts.TerragruntBin = "terragrunt"
	}
	rootAbs, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	// terragrunt relativizes the paths it reports against the resolved
	// working directory; a symlinked root (/var -> /private/var on macOS)
	// would otherwise turn a dependency into a ../../.. chain.
	if rootAbs, err = filepath.EvalSymlinks(rootAbs); err != nil {
		return nil, err
	}
	start := time.Now()
	log := joblog{opts.LogWriter}
	log.banner("terragrunt-atlantis-gen %s", cmp.Or(opts.Version, "dev"))

	output := opts.OutputPath
	if output == "" {
		output = "(stdout)"
	}
	fields := [][2]string{
		{"root", rootAbs},
		// Version first: a long binary path must not push it onto a
		// continuation line, it is what decides discovery semantics.
		{"terragrunt", fmt.Sprintf("%s at %s", terragruntVersion(opts.TerragruntBin), opts.TerragruntBin)},
		{"output", output},
	}
	if len(opts.FilterPaths) > 0 {
		fields = append(fields, [2]string{"filter", strings.Join(opts.FilterPaths, ", ")})
	}
	if opts.BaseRef != "" {
		fields = append(fields, [2]string{"base ref", opts.BaseRef})
	}
	fields = append(fields, [2]string{"settings", opts.settings()})
	log.section("Configuration")
	log.fields(fields)

	log.section("Discovery")
	discoveryStart := time.Now()
	var parseErrs parseErrors
	units, suppressed, err := findUnits(opts.TerragruntBin, rootAbs, rootAbs, "--dependencies", "--include", "--reading", "--exclude")
	if err != nil {
		return nil, err
	}
	parseErrs.add(suppressed, rootAbs, "")
	sourceUnits, suppressed, err := findUnits(opts.TerragruntBin, rootAbs, rootAbs, "--filter", "source=**")
	if err != nil {
		return nil, err
	}
	parseErrs.add(suppressed, rootAbs, "")

	b := &builder{
		opts:      opts,
		gitRoot:   rootAbs + string(filepath.Separator),
		rootAbs:   rootAbs,
		units:     map[string]*Unit{},
		hasSource: map[string]bool{},
		memo:      map[string][]string{},
		building:  map[string]bool{},
	}
	for i := range units {
		if units[i].Type == "unit" {
			b.units[units[i].Path] = &units[i]
		}
	}
	for _, u := range sourceUnits {
		b.hasSource[u.Path] = true
	}

	// Parse errors count only for the units --filter keeps: a config outside
	// the scope cannot change what they watch.
	keep, err := b.filterSet()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(b.units))
	for p := range b.units {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var kept, excluded []string
	for _, path := range paths {
		if keep != nil && !keep[path] {
			continue
		}
		if b.units[path].Exclude.ExcludesPlan() {
			excluded = append(excluded, path)
			continue
		}
		kept = append(kept, path)
	}
	log.line(1, "%s found in %s", plural(len(b.units), "unit"), since(discoveryStart))
	if keep != nil {
		log.line(1, "%s in scope of --filter", plural(len(kept)+len(excluded), "unit"))
	}
	if len(excluded) > 0 {
		log.line(1, "%s excluded by an exclude block covering plan:", plural(len(excluded), "unit"))
		for _, path := range excluded {
			log.line(2, "%s", path)
		}
	}

	if opts.BaseRef != "" {
		eligible := make(map[string]bool, len(kept))
		for _, path := range kept {
			eligible[path] = true
		}
		if err := mergeBaseState(opts, rootAbs, b.units, eligible, &parseErrs); err != nil {
			return nil, err
		}
	}
	if err := parseErrs.report(opts, b.units, keep); err != nil {
		return nil, err
	}

	config := AtlantisConfig{
		Version:       3,
		AutoMerge:     opts.AutoMerge,
		ParallelPlan:  opts.Parallel,
		ParallelApply: opts.Parallel,
	}
	oldConfig, err := readOldConfig(opts.OutputPath)
	if err != nil {
		return nil, err
	}
	if oldConfig != nil && opts.PreserveWorkflows {
		config.Workflows = oldConfig.Workflows
	}
	if oldConfig != nil && opts.PreserveProjects {
		config.Projects = oldConfig.Projects
	}

	parents := 0
	for _, path := range kept {
		project, err := b.createProject(path)
		if err != nil {
			return nil, err
		}
		if project == nil {
			parents++
			continue
		}
		if opts.PreserveProjects {
			updated := false
			for i := range config.Projects {
				if config.Projects[i].Dir == project.Dir {
					config.Projects[i] = *project
					updated = true
					break
				}
			}
			if updated {
				continue
			}
		}
		config.Projects = append(config.Projects, *project)
	}

	sort.Slice(config.Projects, func(i, j int) bool { return config.Projects[i].Dir < config.Projects[j].Dir })

	if opts.ExecutionOrderGroups || opts.DependsOn {
		b.applyExecutionOrder(&config)
	}

	yamlBytes, err := yaml.Marshal(&config)
	if err != nil {
		return nil, err
	}

	log.section("Projects  %d", len(config.Projects))
	if parents > 0 {
		log.para(1, "no project for %s (--ignore-parent-terragrunt)", plural(parents, "parent config"))
	}
	log.projects(config.Projects)
	// The caller still has to persist the bytes, so this reports what was
	// generated and never that the target named in Configuration was written.
	log.section("Summary")
	log.line(1, "%s from %s in %s",
		plural(len(config.Projects), "project"), plural(len(b.units), "unit"), since(start))

	if strings.Contains(runtime.GOOS, "windows") {
		yamlBytes = bytes.ReplaceAll(yamlBytes, []byte("\n"), []byte("\r\n"))
	}
	return yamlBytes, nil
}

// filterSet resolves --filter globs to the set of unit paths to keep;
// nil means no filtering. Globs resolve like TAC's: relative to the
// process working directory.
func (b *builder) filterSet() (map[string]bool, error) {
	if len(b.opts.FilterPaths) == 0 {
		return nil, nil
	}
	keep := map[string]bool{}
	for _, filter := range b.opts.FilterPaths {
		matches, err := filepath.Glob(filter)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			mAbs, err := filepath.Abs(m)
			if err != nil {
				return nil, err
			}
			// rootAbs is symlink-resolved (see Run); a filter reaching the
			// root through a symlink has to compare in the same form.
			if mAbs, err = filepath.EvalSymlinks(mAbs); err != nil {
				return nil, err
			}
			for path := range b.units {
				unitAbs := filepath.Join(b.rootAbs, path)
				if unitAbs == mAbs || strings.HasPrefix(unitAbs, mAbs+string(filepath.Separator)) {
					keep[path] = true
				}
			}
		}
	}
	return keep, nil
}

// dependencies assembles a unit's dependency list (absolute paths) in TAC's
// bucket order: includes, marked reads, dependency blocks, local source,
// cascaded entries, own-dir module sources. Returns nil for skipped parents.
func (b *builder) dependencies(path string) ([]string, error) {
	if deps, ok := b.memo[path]; ok {
		return deps, nil
	}
	if b.building[path] {
		return nil, fmt.Errorf("dependency cycle through %s", path)
	}
	b.building[path] = true
	defer delete(b.building, path)

	u := b.units[path]
	isParent := len(u.Include) == 0 && !b.hasSource[path]
	if isParent && b.opts.IgnoreParentTerragrunt {
		b.memo[path] = nil
		return nil, nil
	}

	unitDirAbs := filepath.Join(b.rootAbs, path)
	var deps []string

	includeSet := map[string]bool{}
	for _, inc := range sortedIncludePaths(u.Include) {
		abs := filepath.Join(b.rootAbs, inc)
		includeSet[abs] = true
		deps = append(deps, abs)
	}

	// Marked reads. Files auto-marked under a local source directory
	// collapse to that directory's globs (TAC's shape); everything else
	// passes through in find's order.
	var sourceDirs []string
	sourceDirSet := map[string]bool{}
	for _, r := range u.Reading {
		abs := filepath.Join(b.rootAbs, r)
		if includeSet[abs] {
			continue
		}
		if isCodeFile(abs) {
			dir := filepath.Dir(abs)
			if dir != unitDirAbs && !sourceDirSet[dir] {
				sourceDirSet[dir] = true
				sourceDirs = append(sourceDirs, dir)
			}
		}
	}
	for _, r := range u.Reading {
		abs := filepath.Join(b.rootAbs, r)
		if includeSet[abs] {
			continue
		}
		// An empty-string mark (`fileexists(...) ? "x" : ""`) resolves to
		// the unit directory itself; TAC filters empty strings out.
		if abs == unitDirAbs {
			continue
		}
		dir := filepath.Dir(abs)
		if sourceDirSet[dir] && (isCodeFile(abs) || strings.HasSuffix(abs, ".hcl")) {
			continue // covered by the source dir globs below
		}
		deps = append(deps, abs)
	}

	// dependency and dependencies blocks. find emits them in map order,
	// which is not deterministic (observed flipping on Terragrunt v1.1.0);
	// sort for stable output.
	if !b.opts.IgnoreDependencyBlocks {
		blockDeps := append([]string(nil), u.Dependencies...)
		sort.Strings(blockDeps)
		for _, dep := range blockDeps {
			deps = append(deps, filepath.Join(b.rootAbs, dep, "terragrunt.hcl"))
		}
	}

	// Local terraform source: the directory globs, then its nested local
	// modules.
	sort.Strings(sourceDirs)
	for _, dir := range sourceDirs {
		deps = append(deps, filepath.Join(dir, "*.tf*"), filepath.Join(dir, "*.tofu*"))
		nested, err := parseTerraformLocalModuleSource(dir)
		if err != nil {
			return nil, err
		}
		deps = append(deps, nested...)
	}

	// Cascade: recurse over dependency-block edges to other units.
	cascaded := make([]string, 0, len(deps)*2)
	seen := map[string]bool{}
	for _, dep := range deps {
		if !seen[dep] {
			seen[dep] = true
			cascaded = append(cascaded, dep)
		}
		if !b.opts.CascadeDependencies {
			continue
		}
		rel, err := filepath.Rel(b.rootAbs, dep)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		depUnit := filepath.ToSlash(filepath.Dir(rel))
		if filepath.Base(rel) != "terragrunt.hcl" {
			continue
		}
		if _, ok := b.units[depUnit]; !ok {
			continue
		}
		childDeps, err := b.dependencies(depUnit)
		if err != nil {
			continue
		}
		for _, childDep := range childDeps {
			if !seen[childDep] {
				seen[childDep] = true
				cascaded = append(cascaded, childDep)
			}
		}
	}

	// The unit's own terraform files may call local modules too.
	own, err := parseTerraformLocalModuleSource(unitDirAbs)
	if err != nil {
		return nil, err
	}
	cascaded = append(cascaded, own...)

	b.memo[path] = cascaded
	return cascaded, nil
}

func (b *builder) createProject(path string) (*AtlantisProject, error) {
	u := b.units[path]
	if u.Exclude.ExcludesPlan() {
		return nil, nil
	}
	deps, err := b.dependencies(path)
	if err != nil {
		return nil, err
	}
	if deps == nil {
		return nil, nil
	}

	unitDirAbs := filepath.Join(b.rootAbs, path)
	absoluteSourceDir := unitDirAbs + string(filepath.Separator)

	relativeDependencies := []string{"*.hcl", "*.tf*", "*.tofu*"}
	for _, dep := range deps {
		rel, err := filepath.Rel(absoluteSourceDir, dep)
		if err != nil {
			return nil, err
		}
		relativeDependencies = append(relativeDependencies, filepath.ToSlash(rel))
	}

	relativeSourceDir := strings.TrimPrefix(absoluteSourceDir, b.gitRoot)
	relativeSourceDir = strings.TrimSuffix(relativeSourceDir, string(filepath.Separator))
	if relativeSourceDir == "" {
		relativeSourceDir = "."
	}

	applyRequirements := &b.opts.DefaultApplyRequirements
	if len(b.opts.DefaultApplyRequirements) == 0 {
		applyRequirements = nil
	}

	project := &AtlantisProject{
		Dir:               filepath.ToSlash(relativeSourceDir),
		Workflow:          b.opts.DefaultWorkflow,
		TerraformVersion:  b.opts.DefaultTerraformVersion,
		ApplyRequirements: applyRequirements,
		Autoplan: AutoplanConfig{
			Enabled:      b.opts.AutoPlan,
			WhenModified: uniqueStrings(relativeDependencies),
		},
	}

	projectName := projectNameRegex.ReplaceAllString(project.Dir, "_")
	if b.opts.CreateProjectName {
		project.Name = projectName
	}
	if b.opts.CreateWorkspace {
		project.Workspace = projectName
	}
	return project, nil
}

// applyExecutionOrder is TAC's fixed-point loop: a project's group is
// max(dependency group)+1, dependencies derived from when_modified — so
// extra deps and cascaded entries count, exactly like TAC.
func (b *builder) applyExecutionOrder(config *AtlantisConfig) {
	projectsMap := make(map[string]*AtlantisProject, len(config.Projects))
	for i := range config.Projects {
		projectsMap[config.Projects[i].Dir] = &config.Projects[i]
	}

	hasChanges := true
	for i := 0; hasChanges && i <= len(config.Projects); i++ {
		hasChanges = false
		for _, project := range config.Projects {
			executionOrderGroup := 0
			dependsOnList := []string{}
			for _, dep := range project.Autoplan.WhenModified {
				depPath := filepath.ToSlash(filepath.Dir(filepath.Join(project.Dir, dep)))
				if depPath == project.Dir {
					continue
				}
				depProject, ok := projectsMap[depPath]
				if !ok {
					continue
				}
				if depProject.ExecutionOrderGroup != nil && *depProject.ExecutionOrderGroup+1 > executionOrderGroup {
					executionOrderGroup = *depProject.ExecutionOrderGroup + 1
				}
				dependsOnList = append(dependsOnList, depProject.Name)
			}
			if projectsMap[project.Dir].ExecutionOrderGroup == nil || *projectsMap[project.Dir].ExecutionOrderGroup != executionOrderGroup {
				if b.opts.ExecutionOrderGroups {
					g := executionOrderGroup
					projectsMap[project.Dir].ExecutionOrderGroup = &g
				}
				if b.opts.DependsOn {
					projectsMap[project.Dir].DependsOn = dependsOnList
				}
				hasChanges = true
			}
		}
	}

	if b.opts.ExecutionOrderGroups {
		sort.Slice(config.Projects, func(i, j int) bool {
			if *config.Projects[i].ExecutionOrderGroup == *config.Projects[j].ExecutionOrderGroup {
				return config.Projects[i].Dir < config.Projects[j].Dir
			}
			return *config.Projects[i].ExecutionOrderGroup < *config.Projects[j].ExecutionOrderGroup
		})
	}
}

func uniqueStrings(str []string) []string {
	if len(str) == 0 {
		return str
	}
	seen := make(map[string]bool)
	result := []string{}
	for _, entry := range str {
		if !seen[entry] {
			seen[entry] = true
			result = append(result, entry)
		}
	}
	return result
}

// readOldConfig preserves parts of an existing output file, like TAC does.
func readOldConfig(outputPath string) (*AtlantisConfig, error) {
	if outputPath == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, nil // not existing yet is the normal first-run case
	}
	config := AtlantisConfig{}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// sortedIncludePaths returns a unit's include paths sorted by path. find
// emits the include object keyed by label in alphabetical order (Terragrunt
// v1.1), so declaration order is already gone; sorting by path keeps the
// output independent of labels too.
func sortedIncludePaths(byLabel map[string]string) []string {
	paths := make([]string, 0, len(byLabel))
	for _, p := range byLabel {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// terragruntVersion asks the binary once, for the job log — the server's
// Terragrunt version decides glob and discovery semantics.
func terragruntVersion(bin string) string {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "unknown version"
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "terragrunt version ")
}
