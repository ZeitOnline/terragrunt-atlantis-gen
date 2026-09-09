# terragrunt-atlantis-gen

Go wrapper around the Terragrunt CLI that produces the same `atlantis.yaml` as
[terragrunt-atlantis-config](https://github.com/ZeitOnline/terragrunt-atlantis-config)
(TAC) and replaces it as the Atlantis pre-workflow hook. Instead of importing
Terragrunt as a library, the wrapper asks the `terragrunt` binary — the same
one that runs the plans — for the structure via `terragrunt find --json`.

## Scope

The wrapper reproduces TAC's output for the surface ZeitOnline actually uses:
dependency discovery, declared file dependencies (as `mark_as_read`), local
module inspection, and the flags the pre-workflow hooks pass.

Skipping a unit is Terragrunt-native: a unit whose evaluated `exclude` block
covers `plan` (actions `all` or `plan`) gets no Atlantis project. The block
lives in the unit — or in an included parent to hide a whole subtree; a child
overrides with `if = false`. Terragrunt evaluates it, the wrapper reads it
from the same `find --json --exclude` call:

```hcl
exclude {
  if      = true
  actions = ["all"]
}
```

Note the native semantics: an `exclude` block also removes the unit from
`terragrunt run --all` — it declares "nothing runs this unit automatically",
not just "Atlantis ignores it". The wrapper inspects the merged `exclude`
field from `find` rather than `--queue-construct-as` filtering, because queue
construction does not honour a child's `if = false` override of an inherited
block (verified on Terragrunt v1.1.0).

The `atlantis_*` settings locals and marker mode are deliberately dropped:

- **All six settings locals** (`atlantis_workflow`,
  `atlantis_terraform_version`, `atlantis_autoplan`, `atlantis_skip`,
  `atlantis_apply_requirements`, `atlantis_project`). Settings come from the
  server-side Atlantis configuration in `atlantis-deployment` (hook flags and
  per-repo config); skipping comes from the `exclude` block. Per-unit
  autoplan has no terragrunt-native counterpart and is unused at ZeitOnline.
  An `atlantis_*` settings local in a repo has no effect on the wrapper.
- **`--project-hcl-files` marker mode.** No ZeitOnline repo uses it.
- **`terragrunt.hcl.json` units.** `terragrunt find` does not discover them;
  convert to HCL first. None exist in our repos.
- **TAC's HCL validation errors** (e.g. a non-string entry in
  `extra_atlantis_dependencies`) — the wrapper reads no HCL, so it cannot
  reproduce them; Terragrunt itself reports broken configs.

The corpus has no cases for dropped features. The `skip` fixture covers the
`exclude` semantics above: inherited from a parent, overridden by a child,
declared in the unit itself, and an `actions` list that leaves `plan` alone.

## Goldens

`testdata/goldens/` holds one golden per case in `internal/goldens/cases.go`:
the `atlantis.yaml` the wrapper must produce, byte for byte. A case is a
fixture under `testdata/fixtures/` plus generate flags, named
`<fixture>[_<flag variant>]`.

The goldens encode the output of TAC **v2.25.1** (ZeitOnline fork) with two
deliberate differences: `when_modified` lists reads in the sorted order
`terragrunt find` reports them, and it includes the files a unit genuinely
reads via `read_terragrunt_config`, which TAC never saw. Bucket order, local
module globs and project naming are TAC's, so a repo switching its
pre-workflow hook sees no other change.

The fixtures derive from TAC's `test/fixtures/` (MIT license in
`testdata/LICENSE-fixtures`), reduced to the wrapper's scope and written the
way a migrated repo looks: a `root.hcl` as the root config, read marks in
`locals`, `exclude` blocks for skipping, var-file mirrors next to
`extra_arguments`.

Regenerating is a deliberate act, never part of a normal test run; the diff
it produces is what gets reviewed:

```sh
go test ./internal/goldens -update
```

## Tests

```sh
go test ./...
```

`internal/goldens` builds the wrapper binary and replays every case through
the real CLI against its golden — byte for byte. Terragrunt ≥ 1.1 must be on
the PATH.

## Migrating a repo

One-off preparation of a Terragrunt repo for the wrapper, done by hand or in
an LLM session, in the same pull request that switches the repo's
pre-workflow hook: TAC loses the declared dependencies the moment its locals
go, so the two must land together.

### 1. Mark dependencies

Every entry of `extra_atlantis_dependencies` becomes a read mark, and the
`extra_atlantis_dependencies` local goes. Marks are plain list entries in a
`locals` block, placed in whichever config owns the knowledge: unit-specific
marks in the unit itself (they may reference the unit's other locals),
subtree-shared marks in the shared included config, whose functions evaluate
per including child. This is the pattern in the `iam` repo:

```hcl
locals {
  # Read-marks: terragrunt find reports these, which is how Atlantis
  # discovery and git-filtered runs know this unit depends on them.
  reads = [
    mark_glob_as_read("${get_repo_root()}/data/humans/**/*.yaml"),
    mark_glob_as_read("${get_repo_root()}/data/humans/*.yaml"),
  ]
}
```

- Entry without glob characters → `mark_as_read(<expression>)`. Function
  calls like `find_in_parent_folders(...)` and conditionals are wrapped as a
  whole.
- Entry with glob characters (`*`, `?`, `[`) → `mark_glob_as_read(<E>)`.
- Fix non-string entries first: `mark_as_read` coerces its argument to a
  string and would silently mark the wrong path.

Files a config already reads natively — `read_terragrunt_config()`,
`read_tfvars_file()`, `sops_decrypt_file()`, a local `terraform.source` —
appear in `reading` on their own and need no mark. `file()` does **not**
register a read (verified on Terragrunt v1.1.0); keep a mark for anything
only `file()` touches.

Traps (verified on Terragrunt v1.1.0):

- **Mark every `**` glob with and without its `**` segment**, as in the
  example above: terragrunt's `**/` requires at least one directory,
  Atlantis's `when_modified` matching requires zero or more — files at the
  glob's top level would silently stop triggering plans otherwise.
- **Do not derive the second pattern with `replace(g, "/**/", "/")`**:
  `replace()` treats a slash-wrapped pattern as a regular expression, and
  `**` is not a valid one. Spell both patterns out, or build the variant
  with `split`/`join`.
- Marks only work in `locals` blocks — inside `inputs`, inside
  `extra_arguments`, or in files nothing includes, they are never evaluated.

### 2. Mirror var-files

`terragrunt find` does not report the var-files of
`terraform { extra_arguments { ... } }`: `required_var_files`,
`optional_var_files`, and `arguments` entries prefixed with `-var-file=`.
Mirror each of these expressions as a read mark in `locals`; the
`extra_arguments` block itself stays as it is:

```hcl
locals {
  # mirrors the var-files in extra_arguments, which find cannot see
  reads = [
    mark_as_read("${get_terragrunt_dir()}/common.tfvars"),
    mark_as_read("${get_terragrunt_dir()}/main.tfvars"), # from "-var-file=..."
  ]
}
```

### 3. Settings locals

`atlantis_skip = true` becomes a native `exclude` block:

```hcl
exclude {
  if      = true
  actions = ["all"]
}
```

Mind the semantic widening: the unit also disappears from
`terragrunt run --all` (see Scope). Where the local sat in an included
parent, the block goes into that parent; a child that set
`atlantis_skip = false` gets its own block with `if = false`.

The other settings locals (`atlantis_workflow`, `atlantis_terraform_version`,
`atlantis_autoplan`, `atlantis_apply_requirements`, `atlantis_project`) are
deleted without replacement — settings come from the server-side Atlantis
configuration.

### 4. Verify

Run the wrapper with the flags from the pre-workflow hook and check that
every path the removed locals declared appears in `when_modified` of the
units that declared it. Where the last TAC-generated `atlantis.yaml` is at
hand, diff the two: apart from the sorted order of `when_modified` and the
files units read via `read_terragrunt_config`, they must match.

```sh
terragrunt-atlantis-gen generate --output atlantis.yaml <hook flags>
```
