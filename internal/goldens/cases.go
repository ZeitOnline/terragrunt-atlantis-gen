// Package goldens defines the parity corpus: every (fixture, flags) pair the
// wrapper must reproduce terragrunt-atlantis-config's (TAC, ZeitOnline fork
// v2.25.1) output for, replayed as CLI invocations of the built binaries.
//
// Case names follow <fixture>[_<flag variant>]; the fixture tree under
// testdata/fixtures is in the migrated state a real repo is in when the
// wrapper runs (read marks, exclude blocks), which TAC reads unchanged.
package goldens

// Case replays one corpus entry as a CLI invocation of `generate`.
type Case struct {
	Name    string   // golden file name: testdata/goldens/<Name>.yaml
	Args    []string // args after `generate --output <tmp>`; paths relative to the repo root
	AbsRoot bool     // resolve the value following --root to an absolute path
	PreSeed string   // written to the output file before the run (preserve-* cases)

	// Diverges names the reason the wrapper's output deliberately differs
	// from TAC's (set-equal when_modified, different bytes). The wrapper
	// golden is then reviewed rather than frozen from TAC, and TAC's own
	// output is kept in testdata/goldens-tac for the TAC gate.
	Diverges string
}

const (
	divergesOrder = "when_modified order: find reports reads sorted, TAC kept declaration order"
	divergesReads = "adds files the units genuinely read (read_terragrunt_config); find reports them, TAC could not"
)

var Cases = []Case{
	{Name: "basic_module", Args: []string{"--root", "testdata/fixtures/basic_module"}},
	{Name: "basic_module_parallel_false", Args: []string{"--root", "testdata/fixtures/basic_module", "--parallel=false"}},
	{Name: "basic_module_autoplan", Args: []string{"--root", "testdata/fixtures/basic_module", "--autoplan"}},
	{Name: "basic_module_automerge", Args: []string{"--root", "testdata/fixtures/basic_module", "--automerge"}},
	{Name: "basic_module_workflow", Args: []string{"--root", "testdata/fixtures/basic_module", "--workflow", "someWorkflow"}},
	{Name: "basic_module_create_workspace", Args: []string{"--root", "testdata/fixtures/basic_module", "--create-workspace"}},
	{Name: "basic_module_apply_requirements", Args: []string{"--root", "testdata/fixtures/basic_module", "--apply-requirements=approved,mergeable"}},
	{Name: "basic_module_root_absolute", Args: []string{"--root", "testdata/fixtures/basic_module"}, AbsRoot: true},
	{Name: "basic_module_root_trailing_slash", Args: []string{"--root", "testdata/fixtures/basic_module/"}},
	// Workflows in a pre-existing output file survive regeneration.
	{Name: "basic_module_preserve_workflows", Args: []string{"--root", "testdata/fixtures/basic_module"}, PreSeed: `workflows:
  terragrunt:
    apply:
      steps:
      - run: terragrunt apply -no-color $PLANFILE
    plan:
      steps:
      - run: terragrunt plan -no-color -out $PLANFILE
`},
	// Projects in a pre-existing output file survive with --preserve-projects.
	{Name: "basic_module_preserve_projects", Args: []string{"--root", "testdata/fixtures/basic_module", "--preserve-projects"}, PreSeed: `projects:
- autoplan:
    enabled: false
    when_modified:
    - '*.hcl'
    - '*.tf*'
  dir: someDir
  name: projectFromPreviousRun
`},

	{Name: "with_parent", Args: []string{"--root", "testdata/fixtures/with_parent"}},
	{Name: "with_parent_ignore_parent_terragrunt_false", Args: []string{"--root", "testdata/fixtures/with_parent", "--ignore-parent-terragrunt=false"}},
	{Name: "with_original_dir", Args: []string{"--root", "testdata/fixtures/with_original_dir"}, Diverges: divergesOrder},
	{Name: "invalid_parent_module", Args: []string{"--root", "testdata/fixtures/invalid_parent_module"}, Diverges: divergesReads},
	{Name: "invalid_parent_module_create_project_name", Args: []string{"--root", "testdata/fixtures/invalid_parent_module", "--create-project-name"}, Diverges: divergesReads},

	{Name: "extra_dependency", Args: []string{"--root", "testdata/fixtures/extra_dependency"}},
	{Name: "extra_arguments", Args: []string{"--root", "testdata/fixtures/extra_arguments"}, Diverges: divergesOrder},
	{Name: "parent_with_extra_deps", Args: []string{"--root", "testdata/fixtures/parent_with_extra_deps"}, Diverges: divergesOrder},

	{Name: "terragrunt_dependency", Args: []string{"--root", "testdata/fixtures/terragrunt_dependency"}},
	{Name: "terragrunt_dependency_ignore_dependency_blocks", Args: []string{"--root", "testdata/fixtures/terragrunt_dependency", "--ignore-dependency-blocks"}},
	{Name: "chained_dependencies", Args: []string{"--root", "testdata/fixtures/chained_dependencies"}},
	{Name: "chained_dependencies_cascade_dependencies_false", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--cascade-dependencies=false"}},
	{Name: "chained_dependencies_execution_order_groups", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--execution-order-groups"}},
	{Name: "chained_dependencies_depends_on", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--depends-on", "--create-project-name"}},
	{Name: "chained_dependencies_execution_order_groups_depends_on", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--execution-order-groups", "--depends-on", "--create-project-name"}},
	{Name: "multi_account_dependencies", Args: []string{"--root", "testdata/fixtures/multi_account_dependencies"}, Diverges: divergesReads},

	{Name: "local_module_source", Args: []string{"--root", "testdata/fixtures/local_module_source"}},
	{Name: "local_module_source_abs", Args: []string{"--root", "testdata/fixtures/local_module_source_abs"}},
	{Name: "local_module_source_nested", Args: []string{"--root", "testdata/fixtures/local_module_source_nested"}},
	{Name: "remote_module_sources", Args: []string{"--root", "testdata/fixtures/remote_module_sources"}},
	{Name: "no_terraform_blocks_autoplan", Args: []string{"--root", "testdata/fixtures/no_terraform_blocks", "--autoplan"}, Diverges: divergesReads},
	{Name: "no_modules", Args: []string{"--root", "testdata/fixtures/no_modules"}},

	{Name: "skip", Args: []string{"--root", "testdata/fixtures/skip"}},

	{Name: "infrastructure_live", Args: []string{"--root", "testdata/fixtures/infrastructure_live"}, Diverges: divergesOrder},
	{Name: "infrastructure_live_filter_prod", Args: []string{"--root", "testdata/fixtures/infrastructure_live", "--filter", "testdata/fixtures/infrastructure_live/prod"}, Diverges: divergesOrder},
	{Name: "infrastructure_live_filter_non_prod", Args: []string{"--root", "testdata/fixtures/infrastructure_live", "--filter", "testdata/fixtures/infrastructure_live/non-prod"}, Diverges: divergesOrder},
	{Name: "infrastructure_live_filter_prod_and_non_prod", Args: []string{"--root", "testdata/fixtures/infrastructure_live", "--filter", "testdata/fixtures/infrastructure_live/non-prod,testdata/fixtures/infrastructure_live/prod"}, Diverges: divergesOrder},
	{Name: "infrastructure_live_filter_glob_mysql", Args: []string{"--root", "testdata/fixtures/infrastructure_live", "--filter", "testdata/fixtures/infrastructure_live/*/*/*/mysql"}, Diverges: divergesOrder},
}
