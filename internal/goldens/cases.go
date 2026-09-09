// Package goldens defines the regression corpus: every (fixture, flags) pair
// the wrapper is gated by, replayed as CLI invocations of the built binary.
//
// Case names follow <fixture>[_<flag variant>]. The fixture tree under
// testdata/fixtures is written the way a migrated repo looks: read marks in
// locals, exclude blocks for skipping, var-file mirrors next to
// extra_arguments.
package goldens

// Case replays one corpus entry as `generate --output <tmp> --root
// testdata/fixtures/<Fixture> <Flags> [--filter ...]`, from the repo root.
type Case struct {
	Name    string   // golden file name: testdata/goldens/<Name>.yaml
	Fixture string   // directory under testdata/fixtures, passed as --root verbatim
	Flags   []string // further generate flags
	Filter  []string // --filter globs, relative to the fixture directory
	AbsRoot bool     // pass --root as an absolute path
	PreSeed string   // written to the output file before the run (preserve-* cases)
}

var Cases = []Case{
	{Name: "basic_module", Fixture: "basic_module"},
	{Name: "basic_module_parallel_false", Fixture: "basic_module", Flags: []string{"--parallel=false"}},
	{Name: "basic_module_autoplan", Fixture: "basic_module", Flags: []string{"--autoplan"}},
	{Name: "basic_module_automerge", Fixture: "basic_module", Flags: []string{"--automerge"}},
	{Name: "basic_module_workflow", Fixture: "basic_module", Flags: []string{"--workflow", "someWorkflow"}},
	{Name: "basic_module_create_workspace", Fixture: "basic_module", Flags: []string{"--create-workspace"}},
	{Name: "basic_module_apply_requirements", Fixture: "basic_module", Flags: []string{"--apply-requirements=approved,mergeable"}},
	{Name: "basic_module_root_absolute", Fixture: "basic_module", AbsRoot: true},
	{Name: "basic_module_root_trailing_slash", Fixture: "basic_module/"},
	// Workflows in a pre-existing output file survive regeneration.
	{Name: "basic_module_preserve_workflows", Fixture: "basic_module", PreSeed: `workflows:
  terragrunt:
    apply:
      steps:
      - run: terragrunt apply -no-color $PLANFILE
    plan:
      steps:
      - run: terragrunt plan -no-color -out $PLANFILE
`},
	// Projects in a pre-existing output file survive with --preserve-projects.
	{Name: "basic_module_preserve_projects", Fixture: "basic_module", Flags: []string{"--preserve-projects"}, PreSeed: `projects:
- autoplan:
    enabled: false
    when_modified:
    - '*.hcl'
    - '*.tf*'
  dir: someDir
  name: projectFromPreviousRun
`},

	// A terragrunt.hcl with neither include nor source counts as a parent
	// config and gets no project unless the hook asks for it.
	{Name: "standalone_unit", Fixture: "standalone_unit"},
	{Name: "standalone_unit_ignore_parent_terragrunt_false", Fixture: "standalone_unit", Flags: []string{"--ignore-parent-terragrunt=false"}},
	{Name: "with_original_dir", Fixture: "with_original_dir"},
	// Labels sort opposite to paths (shared > root, common.hcl < root.hcl), and
	// the two units declare them in opposite order: when_modified must list
	// the includes sorted by path either way.
	{Name: "multiple_includes", Fixture: "multiple_includes"},

	{Name: "extra_dependency", Fixture: "extra_dependency"},
	{Name: "extra_arguments", Fixture: "extra_arguments"},
	{Name: "parent_with_extra_deps", Fixture: "parent_with_extra_deps"},

	{Name: "chained_dependencies", Fixture: "chained_dependencies"},
	{Name: "chained_dependencies_ignore_dependency_blocks", Fixture: "chained_dependencies", Flags: []string{"--ignore-dependency-blocks"}},
	{Name: "chained_dependencies_cascade_dependencies_false", Fixture: "chained_dependencies", Flags: []string{"--cascade-dependencies=false"}},
	{Name: "chained_dependencies_execution_order_groups", Fixture: "chained_dependencies", Flags: []string{"--execution-order-groups"}},
	{Name: "chained_dependencies_depends_on", Fixture: "chained_dependencies", Flags: []string{"--depends-on", "--create-project-name"}},
	{Name: "chained_dependencies_execution_order_groups_depends_on", Fixture: "chained_dependencies", Flags: []string{"--execution-order-groups", "--depends-on", "--create-project-name"}},
	{Name: "multi_account_dependencies", Fixture: "multi_account_dependencies"},

	{Name: "local_module_source", Fixture: "local_module_source"},
	{Name: "remote_module_sources", Fixture: "remote_module_sources"},
	{Name: "no_terraform_blocks_autoplan", Fixture: "no_terraform_blocks", Flags: []string{"--autoplan"}},
	{Name: "no_modules", Fixture: "no_modules"},

	{Name: "skip", Fixture: "skip"},

	{Name: "infrastructure_live", Fixture: "infrastructure_live"},
	{Name: "infrastructure_live_create_project_name", Fixture: "infrastructure_live", Flags: []string{"--create-project-name"}},
	{Name: "infrastructure_live_filter_prod", Fixture: "infrastructure_live", Filter: []string{"prod"}},
	{Name: "infrastructure_live_filter_non_prod", Fixture: "infrastructure_live", Filter: []string{"non-prod"}},
	{Name: "infrastructure_live_filter_prod_and_non_prod", Fixture: "infrastructure_live", Filter: []string{"non-prod", "prod"}},
	{Name: "infrastructure_live_filter_glob_mysql", Fixture: "infrastructure_live", Filter: []string{"*/*/*/mysql"}},
}
