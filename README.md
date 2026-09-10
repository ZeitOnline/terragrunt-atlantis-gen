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

## Deleted and renamed files

Atlantis decides what to plan by matching the pull request's modified files
— deletions and the old name of a rename included — against the
`when_modified` the hook generated from the head checkout. There, a deleted
file no longer matches any `mark_glob_as_read`, and a config that read it
through `read_terragrunt_config()` or a `dependency` block fails to parse,
which `terragrunt find` swallows with exit 0 (the wrapper reads it from the
debug log and fails the hook, see Upstream requests). Either way the file is gone from
`when_modified` and no plan is triggered. TAC did not have this gap: it
wrote the literal glob into the output.

`--base-ref <ref>` closes it. Discovery runs a second time in a detached
worktree of that ref, and every unit's includes, reads and dependency blocks
are the union of both states. The job log names the base commit, what each
unit watches only because of the base, and the units that exist only there.
Those get no project: Atlantis cannot plan a directory that is gone, the
same limit TAC had.

The ref has to resolve in the hook's checkout, which depends on the
server's checkout strategy:

- `merge`: Atlantis clones the base branch in full, so
  `--base-ref "origin/$BASE_BRANCH_NAME"` works as is.
- `branch` (the default): the clone is `--depth=1 --single-branch` of the
  head, so the hook fetches the base first:

  ```sh
  git fetch --depth=1 origin "$BASE_BRANCH_NAME" && terragrunt-atlantis-gen generate --base-ref FETCH_HEAD <hook flags>
  ```

A ref that does not resolve fails the hook instead of silently producing
the head-only config. The `deletions` cases in the corpus show the head-only
output next to the union.

Removing a whole unit is the one deletion no manifest can cover: Atlantis
plans only directories that exist, so a deleted `terragrunt.hcl` produces no
destroy. Destroy first, delete second: with the unit still in place, run
`atlantis plan -d <unit dir> -- -destroy`, apply, and remove the directory
in a follow-up push. Deleting first leaves the state and the resources in
place, and the only signal is the `exists only in <base>: no project` line
in the job log.

## Upstream requests

Three Terragrunt changes would let the wrapper drop workarounds. Each entry
names the interim that applies until the change ships.

- [gruntwork-io/terragrunt#6859](https://github.com/gruntwork-io/terragrunt/issues/6859):
  `find --reading` reporting the pattern of `mark_glob_as_read` next to its
  matches. With it, the wrapper writes the pattern into `when_modified` as
  TAC did, and Atlantis matches deleted files itself. `--base-ref` then only
  covers natively read files a pull request deletes, and the second,
  `**`-less pattern from the runbook is no longer needed for Atlantis
  (Terragrunt's own `--filter-affected` still needs it). Interim:
  `--base-ref`, see above.
- [gruntwork-io/terragrunt#6858](https://github.com/gruntwork-io/terragrunt/issues/6858):
  discovery following `module` blocks with a local `source` inside a unit's
  local `terraform.source` directory, recursively, and recording those
  directories as read. With it, nested modules arrive in `reading` and
  `internal/generate/tfmodules.go`, the port of TAC's `parse_tf.go` that
  scans Terraform code for exactly these calls, goes. Interim: that port,
  which emits `<dir>/*.tf*` globs for every nested local module.
- [gruntwork-io/terragrunt#6856](https://github.com/gruntwork-io/terragrunt/issues/6856):
  `find` reporting suppressed parse errors at WARN, plus an opt-in that fails
  on them. With it, the wrapper passes that option instead of reading the
  debug log. Interim: every discovery runs with `--log-level debug
  --log-format json`, and the wrapper reports each suppressed error in the
  job log as `terragrunt suppressed a parse error: <file>:<pos>: <diagnostic>`,
  followed by a count, and fails the hook, TAC's behaviour;
  `--fail-on-parse-errors=false` keeps the run going with the log lines
  only. This rides on the wording of terragrunt's debug
  message, which `TestSuppressedParseErrors` pins against the Terragrunt
  version CI runs. `terragrunt hcl validate` is no substitute: it resolves
  `dependency` outputs, so it needs state access on every hook run, and it
  fails on terraform-infra's cluster-infra configs with an output-parsing
  error.

## Development

Go, [Task](https://taskfile.dev/) and Terragrunt ≥ 1.1 on the PATH; the git
hooks need [lefthook](https://lefthook.dev/), installed once with `task setup`.

```sh
task --list             # every task
task build              # ./terragrunt-atlantis-gen, --version reports git describe
task lint               # gofmt, go vet, staticcheck, no Terragrunt import
task test               # go test, goldens included; task test -- -run TestGoldens
task release:snapshot   # every release artifact into dist/, nothing published
```

CI runs the same tasks: `test.yaml` runs `task lint` and `task test` on every
pull request and push to `main`, `release.yaml` runs `task release` once
release-please has cut the tag. Tool versions the tasks pin (staticcheck) sit
in `Taskfile.yaml` under a `# renovate:` comment, so Renovate bumps them.

## Goldens

`testdata/goldens/` holds one golden per case in `internal/goldens/cases.go`:
the `atlantis.yaml` the wrapper must produce, byte for byte. A case is a
fixture under `testdata/fixtures/` plus generate flags, named
`<fixture>[_<flag variant>]`.

The goldens encode the output of TAC **v2.25.1** (ZeitOnline fork) with two
deliberate differences: `when_modified` lists includes and reads sorted by
path — `terragrunt find` reports neither in declaration order — and it
includes the files a unit genuinely reads via `read_terragrunt_config`, which
TAC never saw. Bucket order, local module globs and project naming are TAC's,
so a repo switching its pre-workflow hook sees no other change.

The fixtures derive from TAC's `test/fixtures/` (MIT license in
`testdata/LICENSE-fixtures`), reduced to the wrapper's scope and written the
way a migrated repo looks: a `root.hcl` as the root config, read marks in
`locals`, `exclude` blocks for skipping, var-file mirrors next to
`extra_arguments`.

Regenerating is a deliberate act, never part of a normal test run; the diff
it produces is what gets reviewed:

```sh
task goldens:update
```

## Tests

```sh
task test
```

`internal/goldens` builds the wrapper binary and replays every case through
the real CLI against its golden — byte for byte. Terragrunt ≥ 1.1 must be on
the PATH. The history cases run in temporary repositories, where a
version-manager shim that only resolves inside configured directories does
not work: for mise, `task test` points `TERRAGRUNT_BIN` at the binary behind
the shim itself; for any other manager, set it by hand. The test log names
the binary and version the goldens replayed against.

`task test` runs with `-count=1`: the goldens exec Terragrunt from a
subprocess, which Go's test cache never sees, so a cached pass could outlive
the binary it was made with.

CI does not trust the runner's Terragrunt. `test.yaml` downloads the release
pinned in `TERRAGRUNT_VERSION`, kept at the version the Atlantis image runs,
verifies that it is the one on the PATH, and only then runs `task test`.
Renovate bumps the pin when Terragrunt releases, so a green bump PR here
certifies the wrapper for that version before the Atlantis image moves.

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
- **One `locals` block per config.** `find` suppresses parse errors during
  discovery, so a duplicate `locals` block silently drops every mark in the
  file — merge marks into the existing block.
- **`find` does not mark an include-inherited local `terraform.source`**
  (its `source=**` filter resolves it, its read-marking does not; Terragrunt
  v1.1). Mark the module tree explicitly in the config that declares the
  source.

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
