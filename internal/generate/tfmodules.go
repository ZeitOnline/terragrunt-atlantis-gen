package generate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// Ported from TAC's parse_tf.go. `terragrunt find --reading` lists what a
// unit's config reads — includes, marks, the files of a local
// terraform.source — but Terragrunt (v1.1) never opens Terraform code, so a
// `module` block with a relative source (./, ../) is invisible to it. The
// module still has to land in when_modified: otherwise a change to a shared
// module triggers no plan, and Atlantis gives no error for a missing path.
// Applies to a unit's local source directory and to the unit's own
// directory (units that keep their code in place have no source at all),
// recursively, since modules call modules; emitted as <dir>/*.tf* globs,
// the shape TAC produced. This is how iam and terraform-infra share
// modules (modules/gcp/humans calls ../iam-memberships).

var localModuleSourcePrefixes = []string{"./", "../", ".\\", "..\\"}

func parseTerraformLocalModuleSource(path string) ([]string, error) {
	moduleCallSources, err := extractModuleCallSources(path)
	if err != nil {
		return nil, err
	}

	sourceMap := make(map[string]struct{})
	for _, source := range moduleCallSources {
		if !isLocalTerraformModuleSource(source) {
			continue
		}
		modulePath := filepath.Join(path, source)
		sourceMap[filepath.Join(modulePath, "*.tf*")] = struct{}{}
		sourceMap[filepath.Join(modulePath, "*.tofu*")] = struct{}{}

		subSources, err := parseTerraformLocalModuleSource(modulePath)
		if err != nil {
			return nil, err
		}
		for _, s := range subSources {
			sourceMap[s] = struct{}{}
		}
	}

	sources := make([]string, 0, len(sourceMap))
	for s := range sourceMap {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	return sources, nil
}

func isLocalTerraformModuleSource(source string) bool {
	for _, prefix := range localModuleSourcePrefixes {
		if strings.HasPrefix(source, prefix) {
			return true
		}
	}
	return false
}

var moduleSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{{Type: "module", LabelNames: []string{"name"}}},
}

var sourceSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{{Name: "source"}},
}

func extractModuleCallSources(dir string) ([]string, error) {
	var files []string
	for _, pattern := range []string{"*.tf", "*.tf.json", "*.tofu", "*.tofu.json"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}

	parser := hclparse.NewParser()
	var sources []string
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var f *hcl.File
		var diags hcl.Diagnostics
		if strings.HasSuffix(file, ".json") {
			f, diags = parser.ParseJSON(content, file)
		} else {
			f, diags = parser.ParseHCL(content, file)
		}
		if diags.HasErrors() {
			continue
		}
		content2, _, _ := f.Body.PartialContent(moduleSchema)
		for _, block := range content2.Blocks {
			attrs, _, _ := block.Body.PartialContent(sourceSchema)
			if attr, ok := attrs.Attributes["source"]; ok {
				if val, d := attr.Expr.Value(nil); !d.HasErrors() && val.Type() == cty.String {
					sources = append(sources, val.AsString())
				}
			}
		}
	}
	return sources, nil
}
