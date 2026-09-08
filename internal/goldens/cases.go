// Package goldens defines the frozen TAC test corpus: every (fixture, flags)
// pair from terragrunt-atlantis-config's cmd/generate_test.go at v2.25.1,
// replayed as CLI invocations of the released binary.
//
// Generated from cmd/generate_test.go (ZeitOnline fork, tag v2.25.1); the
// hand-written entries at the end cover the tests that are not plain
// runTest calls. Do not edit the generated block by hand.
package goldens

// Case replays one TAC test as a CLI invocation of `generate`.
type Case struct {
	Name    string   // golden file name: testdata/goldens/<Name>.yaml (or .err)
	Args    []string // args after `generate --output <tmp>`; paths relative to the repo root
	AbsRoot bool     // resolve the value following --root to an absolute path
	Chdir   string   // working directory for the run, relative to the repo root ("" = repo root)
	PreSeed string   // written to the output file before the run (preserve-* tests)
	WantErr bool     // expect a non-zero exit; golden holds stderr instead of the output file

	// Diverges names the reason the wrapper's output deliberately differs
	// from TAC's golden (set-equal when_modified, different bytes). The case
	// is then gated by a reviewed wrapper golden in testdata/goldens-wrapper.
	Diverges string

	// Unsupported names the reason a case is outside the wrapper's scope.
	// The golden still documents TAC's behaviour and the freezer still
	// writes it, but the parity suite skips the case. See README, "Scope".
	Unsupported string
}

var Cases = []Case{
	{Name: "TestSettingRoot", Args: []string{"--root", "testdata/fixtures/basic_module"}},
	{Name: "TestWithParallelizationDisabled", Args: []string{"--root", "testdata/fixtures/basic_module", "--parallel=false"}},
	{Name: "TestIgnoringParentTerragrunt", Args: []string{"--root", "testdata/fixtures/with_parent"}},
	{Name: "TestNotIgnoringParentTerragrunt", Args: []string{"--root", "testdata/fixtures/with_parent", "--ignore-parent-terragrunt=false"}},
	{Name: "TestEnablingAutoplan", Args: []string{"--root", "testdata/fixtures/basic_module", "--autoplan"}},
	{Name: "TestSettingWorkflowName", Args: []string{"--root", "testdata/fixtures/basic_module", "--workflow", "someWorkflow"}},
	{Name: "TestExtraDeclaredDependencies", Args: []string{"--root", "testdata/fixtures/extra_dependency"}},
	{Name: "TestLocalTerraformModuleSource", Args: []string{"--root", "testdata/fixtures/local_terraform_module_source"}},
	{Name: "TestLocalTerraformAbsModuleSource", Args: []string{"--root", "testdata/fixtures/local_terraform_abs_module_source"}},
	{Name: "TestLocalTfModuleSource", Args: []string{"--root", "testdata/fixtures/local_tf_module_source"}},
	{Name: "TestTerragruntDependencies", Args: []string{"--root", "testdata/fixtures/terragrunt_dependency"}},
	{Name: "TestIgnoringTerragruntDependencies", Args: []string{"--root", "testdata/fixtures/terragrunt_dependency", "--ignore-dependency-blocks"}},
	{Name: "TestCustomWorkflowName", Args: []string{"--root", "testdata/fixtures/different_workflow_names"}, Unsupported: "settings locals are dropped; settings come from the server-side Atlantis config"},
	{Name: "TestUnparseableParent", Args: []string{"--root", "testdata/fixtures/invalid_parent_module"}, Diverges: "adds files the units genuinely read (read_terragrunt_config); find reports them, TAC could not"},
	{Name: "TestWithWorkspaces", Args: []string{"--root", "testdata/fixtures/basic_module", "--create-workspace"}},
	{Name: "TestWithProjectNames", Args: []string{"--root", "testdata/fixtures/invalid_parent_module", "--create-project-name"}, Diverges: "adds files the units genuinely read (read_terragrunt_config); find reports them, TAC could not"},
	{Name: "TestMergingLocalDependenciesFromParent", Args: []string{"--root", "testdata/fixtures/parent_with_extra_deps"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestWorkflowFromParentInLocals", Args: []string{"--root", "testdata/fixtures/parent_with_workflow_local"}, Unsupported: "settings locals are dropped; settings come from the server-side Atlantis config"},
	{Name: "TestChildWorkflowOverridesParentWorkflow", Args: []string{"--root", "testdata/fixtures/child_and_parent_specify_workflow"}, Unsupported: "settings locals are dropped; settings come from the server-side Atlantis config"},
	{Name: "TestExtraArguments", Args: []string{"--root", "testdata/fixtures/extra_arguments"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestInfrastructureLive", Args: []string{"--root", "testdata/fixtures/terragrunt-infrastructure-live-example"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestModulesWithNoTerraformSourceDefinitions", Args: []string{"--root", "testdata/fixtures/no_terraform_blocks", "--parallel", "--autoplan"}, Diverges: "adds files the units genuinely read (read_terragrunt_config); find reports them, TAC could not"},
	{Name: "TestInfrastructureMutliAccountsVPCRoute53TGWCascading", Args: []string{"--root", "testdata/fixtures/multi_accounts_vpc_route53_tgw", "--cascade-dependencies"}, Diverges: "adds files the units genuinely read (read_terragrunt_config); find reports them, TAC could not"},
	{Name: "TestAutoPlan", Args: []string{"--root", "testdata/fixtures/autoplan", "--autoplan=false"}, Unsupported: "per-unit autoplan is dropped: no terragrunt-native counterpart, and unused at ZeitOnline"},
	{Name: "TestSkippingModules", Args: []string{"--root", "testdata/fixtures/skip"}},
	{Name: "TestTerraformVersionConfig", Args: []string{"--root", "testdata/fixtures/terraform_version", "--terraform-version", "0.14.9001"}, Unsupported: "settings locals are dropped; settings come from the server-side Atlantis config"},
	{Name: "TestEnablingAutomerge", Args: []string{"--root", "testdata/fixtures/basic_module", "--automerge"}},
	{Name: "TestChainedDependencies", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--cascade-dependencies"}},
	{Name: "TestChainedDependenciesHiddenBehindFlag", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--cascade-dependencies=false"}},
	{Name: "TestApplyRequirementsLocals", Args: []string{"--root", "testdata/fixtures/apply_requirements_overrides"}, Unsupported: "settings locals are dropped; settings come from the server-side Atlantis config"},
	{Name: "TestApplyRequirementsFlag", Args: []string{"--root", "testdata/fixtures/basic_module", "--apply-requirements=approved,mergeable"}},
	{Name: "TestFilterFlagWithInfraLiveProd", Args: []string{"--root", "testdata/fixtures/terragrunt-infrastructure-live-example", "--filter", "testdata/fixtures/terragrunt-infrastructure-live-example/prod"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestFilterFlagWithInfraLiveNonProd", Args: []string{"--root", "testdata/fixtures/terragrunt-infrastructure-live-example", "--filter", "testdata/fixtures/terragrunt-infrastructure-live-example/non-prod"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestFilterFlagWithInfraLiveProdAndNonProd", Args: []string{"--root", "testdata/fixtures/terragrunt-infrastructure-live-example", "--filter", "testdata/fixtures/terragrunt-infrastructure-live-example/non-prod,testdata/fixtures/terragrunt-infrastructure-live-example/prod"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestFilterGlobFlagWithInfraLiveMySql", Args: []string{"--root", "testdata/fixtures/terragrunt-infrastructure-live-example", "--filter", "testdata/fixtures/terragrunt-infrastructure-live-example/*/*/*/mysql"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestMultipleIncludes", Args: []string{"--root", "testdata/fixtures/multiple_includes", "--terraform-version", "0.14.9001"}, Unsupported: "settings locals are dropped; settings come from the server-side Atlantis config"},
	{Name: "TestRemoteModuleSourceBitbucket", Args: []string{"--root", "testdata/fixtures/remote_module_source_bitbucket"}},
	{Name: "TestRemoteModuleSourceGCS", Args: []string{"--root", "testdata/fixtures/remote_module_source_gcs"}},
	{Name: "TestRemoteModuleSourceGitHTTPS", Args: []string{"--root", "testdata/fixtures/remote_module_source_git_https"}},
	{Name: "TestRemoteModuleSourceGitSCPLike", Args: []string{"--root", "testdata/fixtures/remote_module_source_git_scp_like"}},
	{Name: "TestRemoteModuleSourceGitSSH", Args: []string{"--root", "testdata/fixtures/remote_module_source_git_ssh"}},
	{Name: "TestRemoteModuleSourceGithubHTTPS", Args: []string{"--root", "testdata/fixtures/remote_module_source_github_https"}},
	{Name: "TestRemoteModuleSourceGithubSSH", Args: []string{"--root", "testdata/fixtures/remote_module_source_github_ssh"}},
	{Name: "TestRemoteModuleSourceHTTP", Args: []string{"--root", "testdata/fixtures/remote_module_source_http"}},
	{Name: "TestRemoteModuleSourceHTTPS", Args: []string{"--root", "testdata/fixtures/remote_module_source_https"}},
	{Name: "TestRemoteModuleSourceMercurial", Args: []string{"--root", "testdata/fixtures/remote_module_source_mercurial"}},
	{Name: "TestRemoteModuleSourceS3", Args: []string{"--root", "testdata/fixtures/remote_module_source_s3"}},
	{Name: "TestRemoteModuleSourceTerraformRegistry", Args: []string{"--root", "testdata/fixtures/remote_module_source_terraform_registry"}},
	{Name: "TestEnvHCLProjectsWithDeps", Args: []string{"--root", "testdata/fixtures/proj_hcl_with_external_deps/my-stack", "--cascade-dependencies", "--project-hcl-files=stack.hcl", "--create-hcl-project-childs=false", "--create-hcl-project-external-childs=false"}, Unsupported: "project-hcl-files mode is not used at ZeitOnline"},
	{Name: "TestEnvHCLProjectsNoChilds", Args: []string{"--root", "testdata/fixtures", "--project-hcl-files=env.hcl", "--create-hcl-project-childs=false", "--create-hcl-project-external-childs=false"}, Unsupported: "project-hcl-files mode is not used at ZeitOnline"},
	{Name: "TestEnvHCLProjectsSubChilds", Args: []string{"--root", "testdata/fixtures", "--project-hcl-files=env.hcl", "--create-hcl-project-childs=true", "--create-hcl-project-external-childs=false"}, Unsupported: "project-hcl-files mode is not used at ZeitOnline"},
	{Name: "TestEnvHCLProjectsExternalChilds", Args: []string{"--root", "testdata/fixtures", "--project-hcl-files=env.hcl", "--create-hcl-project-childs=false", "--create-hcl-project-external-childs=true"}, Unsupported: "project-hcl-files mode is not used at ZeitOnline"},
	{Name: "TestEnvHCLProjectsAllChilds", Args: []string{"--root", "testdata/fixtures", "--project-hcl-files=env.hcl", "--create-hcl-project-childs=true", "--create-hcl-project-external-childs=true"}, Unsupported: "project-hcl-files mode is not used at ZeitOnline"},
	{Name: "TestEnvHCLProjectMarker", Args: []string{"--root", "testdata/fixtures/project_hcl_with_project_marker", "--project-hcl-files=env.hcl", "--use-project-markers=true"}, Unsupported: "project-hcl-files mode is not used at ZeitOnline"},
	{Name: "TestWithOriginalDir", Args: []string{"--root", "testdata/fixtures/with_original_dir"}, Diverges: "when_modified order: find reports reads sorted, TAC kept declaration order"},
	{Name: "TestWithExecutionOrderGroups", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--execution-order-groups"}},
	{Name: "TestWithExecutionOrderGroupsAndDependsOn", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--execution-order-groups", "--depends-on", "--create-project-name"}},
	{Name: "TestWithDependsOn", Args: []string{"--root", "testdata/fixtures/chained_dependencies", "--depends-on", "--create-project-name"}},

	// --root as an absolute path (resolved at runtime).
	{Name: "TestRootPathBeingAbsolute", Args: []string{"--root", "testdata/fixtures/basic_module"}, AbsRoot: true},
	// --root with a trailing separator.
	{Name: "TestRootPathHavingTrailingSlash", Args: []string{"--root", "testdata/fixtures/basic_module/"}},
	// --root "." in a directory without terragrunt files; the positional arg is
	// inert (cobra arbitrary args) and kept verbatim from the original test.
	{Name: "TestWithNoTerragruntFiles", Args: []string{"--root", ".", "testdata/fixtures/no_modules"}, Chdir: "internal/goldens"},
	// Non-string entry in extra_atlantis_dependencies aborts the run.
	{Name: "TestNonStringErrorOnExtraDeclaredDependencies", Args: []string{"--root", "testdata/fixtures_errors/extra_dependency_error"}, WantErr: true, Unsupported: "the wrapper reads no HCL; TAC's non-string validation error is not reproducible"},
	// Workflows in a pre-existing output file survive regeneration.
	{Name: "TestPreservingOldWorkflows", Args: []string{"--root", "testdata/fixtures/basic_module"}, PreSeed: `workflows:
  terragrunt:
    apply:
      steps:
      - run: terragrunt apply -no-color $PLANFILE
    plan:
      steps:
      - run: terragrunt plan -no-color -out $PLANFILE
`},
	// Projects in a pre-existing output file survive with --preserve-projects.
	{Name: "TestPreservingOldProjects", Args: []string{"--preserve-projects", "--root", "testdata/fixtures/basic_module"}, PreSeed: `projects:
- autoplan:
    enabled: false
    when_modified:
    - '*.hcl'
    - '*.tf*'
  dir: someDir
  name: projectFromPreviousRun
`},
}
