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

The parity cases covering dropped features are annotated with `Unsupported`
in `internal/goldens/cases.go` (14 of 64) and skipped by the suite; their
goldens stay as documentation of TAC's behaviour. Skipping stays gated by
TAC's own skip fixture (TestSkippingModules), whose migrated form adds
`exclude` blocks next to the `atlantis_skip` locals.

## Goldens

`testdata/goldens/` freezes the behaviour of TAC **v2.25.1** (ZeitOnline fork,
release binary `darwin_arm64`, checksum-verified): one golden file per
(fixture, flags) pair from TAC's `cmd/generate_test.go`, produced by real CLI
invocations. They define "same as TAC" for this project and only change
deliberately.

- `testdata/fixtures/`, `testdata/fixtures_errors/`: taken unchanged from
  TAC v2.25.1 (`test/fixtures/`, MIT license in `testdata/LICENSE-fixtures`).
- `internal/goldens/cases.go`: the 64 cases, generated from
  `generate_test.go`; the fork's four pure unit tests of internal helpers are
  deliberately not included.
- Re-freezing (only ever deliberately, never as part of a normal test run):

  ```sh
  go run ./tools/freeze -tac /path/to/terragrunt-atlantis-config
  ```

`testdata/migrated/` is the same fixture tree after the one-off migration
described below (marks, var-file mirrors, `exclude` blocks in the skip
fixture) — the state a real repo is in when the wrapper runs. The parity
suite replays the wrapper against it; `TestMigratedFixturesMatchTACGoldens`
(gated on `TAC_BIN`) proves it stays additive by replaying TAC over it
against the same goldens.

## Tests

```sh
go test ./...
```

`internal/goldens` builds the wrapper binary and replays every supported case
through the real CLI against its golden — byte for byte. Terragrunt ≥ 1.1
must be on the PATH. Twelve cases carry a `Diverges` annotation: their output
is set-equal but not byte-equal to TAC's (when_modified order — find reports
reads sorted — and files the units genuinely read via read_terragrunt_config,
which TAC never saw). Each is gated by a reviewed golden in
`testdata/goldens-wrapper/`, frozen deliberately with:

```sh
go run ./tools/freeze -wrapper /path/to/terragrunt-atlantis-gen
```

## Migrating a repo

One-off preparation of a Terragrunt repo for the wrapper, done by hand or in
an LLM session. The change is additive: everything TAC reads stays in place —
TAC and the wrapper run side by side on the same state. Cleaning up (removing
the locals) only happens after TAC has been retired.

### 1. Mark dependencies

Wrap every entry of `extra_atlantis_dependencies`:

- Entry without glob characters → `mark_as_read(<expression>)`. Function
  calls like `find_in_parent_folders(...)` and conditionals are wrapped as a
  whole.
- Entry whose literal text contains glob characters (`*`, `?`, `[`) →
  `try(mark_glob_as_read(<E>), <E>)`. The Terragrunt embedded in TAC
  (v0.86.2) does not know `mark_glob_as_read`; `try()` falls back to the bare
  string there, while the server-side Terragrunt (≥ 1.1) records the glob as
  read. The same pattern is already in production in the `iam` repo.
- Leave already-wrapped entries (`mark_as_read`, `mark_glob_as_read`, `try`)
  untouched.
- Fix non-string entries first: `mark_as_read` would coerce the value to a
  string and silently swallow TAC's error about it.

Marks only work in `locals` blocks — inside `inputs`, inside
`extra_arguments`, or in files nothing includes, they are never evaluated.

### 2. Mirror var-files

TAC reads three sources from `terraform { extra_arguments { ... } }`:
`required_var_files`, `optional_var_files`, and `arguments` entries prefixed
with `-var-file=`. Mirror each of these expressions as a read mark in
`locals`; leave the `extra_arguments` block itself alone:

```hcl
locals {
  atlantis_var_file_reads = [
    mark_as_read("${get_terragrunt_dir()}/common.tfvars"),
    mark_as_read("${get_terragrunt_dir()}/main.tfvars"), # from "-var-file=..."
  ]
}
```

### 3. Settings locals

`atlantis_skip = true` gets a native `exclude` block **added next to it** —
the local stays in place for TAC during the shadow phase and is removed after
TAC is retired:

```hcl
locals {
  atlantis_skip = true # removed at cutover
}

exclude {
  if      = true
  actions = ["all"]
}
```

Mind the semantic widening: the unit also disappears from
`terragrunt run --all` (see Scope). Where the local sits in an included
parent, the block goes into that parent; a child that set
`atlantis_skip = false` gets its own block with `if = false`.

The other settings locals (`atlantis_workflow`, `atlantis_terraform_version`,
`atlantis_autoplan`, `atlantis_apply_requirements`, `atlantis_project`) are
dropped — settings come from the server-side Atlantis configuration; nothing
to migrate.

### 4. Verify

Run TAC with the flags from the pre-workflow hook before and after the
migration; identical output proves the migration was additive:

```sh
terragrunt-atlantis-config generate --output before.yaml <hook flags>
# ... migrate ...
terragrunt-atlantis-config generate --output after.yaml <hook flags>
diff before.yaml after.yaml
```
