// Package generate produces the atlantis.yaml that terragrunt-atlantis-config
// (TAC) would produce, from `terragrunt find --json` instead of parsed HCL.
// The output logic is ported from TAC v2.25.1 cmd/generate.go so the result
// stays byte-compatible; the goldens in testdata/goldens are the contract.
package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

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

	units, err := findUnits(opts.TerragruntBin, rootAbs, "--dependencies", "--include", "--reading", "--exclude")
	if err != nil {
		return nil, err
	}
	sourceUnits, err := findUnits(opts.TerragruntBin, rootAbs, "--filter", "source=**")
	if err != nil {
		return nil, err
	}

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

	keep, err := b.filterSet()
	if err != nil {
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

	paths := make([]string, 0, len(b.units))
	for p := range b.units {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		if keep != nil && !keep[path] {
			continue
		}
		project, err := b.createProject(path)
		if err != nil {
			return nil, err
		}
		if project == nil {
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

	// Includes, in declaration order.
	includes, err := orderedIncludePaths(u.Include)
	if err != nil {
		return nil, err
	}
	includeSet := map[string]bool{}
	for _, inc := range includes {
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

// orderedIncludePaths returns the include paths in declaration order. The
// JSON object's key order carries it; map decoding would destroy it.
func orderedIncludePaths(raw map[string]string) ([]string, error) {
	// A single include needs no ordering; the common case.
	if len(raw) <= 1 {
		for _, p := range raw {
			return []string{p}, nil
		}
		return nil, nil
	}
	paths := make([]string, 0, len(raw))
	for _, p := range raw {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}
