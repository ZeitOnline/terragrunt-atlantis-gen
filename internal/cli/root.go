// Package cli wires the terragrunt-atlantis-gen commands. The generate
// command carries terragrunt-atlantis-config's flag surface so the Atlantis
// pre-workflow hook line works unchanged.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ZeitOnline/terragrunt-atlantis-gen/internal/generate"
)

// Main runs the root command and returns the process exit code.
func Main(version string) int {
	if err := newRootCmd(version).Execute(); err != nil {
		return 1
	}
	return 0
}

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:          "terragrunt-atlantis-gen",
		Short:        "Generate atlantis.yaml for Terragrunt repos via the Terragrunt CLI",
		Version:      version,
		SilenceUsage: true,
	}
	root.AddCommand(newGenerateCmd(version))
	return root
}

func newGenerateCmd(version string) *cobra.Command {
	var opts generate.Options
	var numExecutors int64
	var createParentProject bool
	var projectHclFiles []string
	var useProjectMarkers bool

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Makes atlantis config (drop-in replacement for terragrunt-atlantis-config)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.DependsOn && !opts.CreateProjectName {
				return fmt.Errorf("--depends-on requires --create-project-name")
			}
			// Refusing beats silently producing different output.
			if len(projectHclFiles) > 0 || useProjectMarkers {
				return fmt.Errorf("--project-hcl-files mode is not supported; see the Scope section of the README")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.LogWriter = cmd.ErrOrStderr()
			fmt.Fprintf(opts.LogWriter, "terragrunt-atlantis-gen %s\n", version)
			yamlBytes, err := generate.Run(opts)
			if err != nil {
				return err
			}
			if opts.OutputPath != "" {
				return os.WriteFile(opts.OutputPath, yamlBytes, 0o644)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(yamlBytes))
			return nil
		},
	}

	pwd, err := os.Getwd()
	if err != nil {
		pwd = "."
	}

	f := cmd.PersistentFlags()
	f.BoolVar(&opts.AutoPlan, "autoplan", false, "Enable auto plan. Default is disabled")
	f.BoolVar(&opts.AutoMerge, "automerge", false, "Enable auto merge. Default is disabled")
	f.BoolVar(&opts.IgnoreParentTerragrunt, "ignore-parent-terragrunt", true, "Ignore parent terragrunt configs (those which don't reference a terraform module). Default is enabled")
	f.BoolVar(&createParentProject, "create-parent-project", false, "Accepted for compatibility; terragrunt-atlantis-config never reads it")
	f.BoolVar(&opts.IgnoreDependencyBlocks, "ignore-dependency-blocks", false, "When true, dependencies found in `dependency` blocks will be ignored")
	f.BoolVar(&opts.Parallel, "parallel", true, "Enables plans and applys to happen in parallel. Default is enabled")
	f.BoolVar(&opts.CreateWorkspace, "create-workspace", false, "Use different workspace for each project. Default is use default workspace")
	f.BoolVar(&opts.CreateProjectName, "create-project-name", false, "Add different name for each project. Default is false")
	f.BoolVar(&opts.PreserveWorkflows, "preserve-workflows", true, "Preserves workflows from old output files. Default is true")
	f.BoolVar(&opts.PreserveProjects, "preserve-projects", false, "Preserves projects from old output files to enable incremental builds. Default is false")
	f.BoolVar(&opts.CascadeDependencies, "cascade-dependencies", true, "When true, dependencies will cascade transitively. Default is true")
	f.StringVar(&opts.DefaultWorkflow, "workflow", "", "Name of the workflow to be customized in the atlantis server. Default is to not set")
	f.StringSliceVar(&opts.DefaultApplyRequirements, "apply-requirements", []string{}, "Requirements that must be satisfied before `atlantis apply` can be run")
	f.StringVar(&opts.OutputPath, "output", "", "Path of the file where configuration will be generated. Default is stdout")
	f.StringSliceVar(&opts.FilterPaths, "filter", []string{}, "Comma-separated paths or glob expressions to scope down the config")
	f.StringVar(&opts.Root, "root", pwd, "Path to the root directory of the git repo you want to build config for. Default is current dir")
	f.StringVar(&opts.DefaultTerraformVersion, "terraform-version", "", "Default terraform version to specify for all modules")
	f.Int64Var(&numExecutors, "num-executors", 15, "Accepted for compatibility; discovery is one terragrunt process")
	f.StringSliceVar(&projectHclFiles, "project-hcl-files", nil, "Not supported; fails loudly")
	f.BoolVar(&useProjectMarkers, "use-project-markers", false, "Not supported; fails loudly")
	f.BoolVar(&opts.ExecutionOrderGroups, "execution-order-groups", false, "Computes execution_order_groups for projects")
	f.BoolVar(&opts.DependsOn, "depends-on", false, "Computes depends_on for projects. Requires --create-project-name.")
	f.StringVar(&opts.TerragruntBin, "terragrunt-bin", "terragrunt", "The terragrunt binary to ask for the repo structure")
	f.BoolVar(&opts.FailOnParseErrors, "fail-on-parse-errors", true, "Fail when terragrunt find suppressed a parse error in a unit the run keeps: the unit keeps its project but watches fewer paths. Default is enabled; =false only reports them in the log")
	f.StringVar(&opts.BaseRef, "base-ref", "", "Git ref of the pull request's base (origin/main, FETCH_HEAD). Discovery runs again in a worktree of that ref so when_modified also lists what a unit read there, and files the pull request deletes or renames still trigger its plan. Default is off")

	return cmd
}
