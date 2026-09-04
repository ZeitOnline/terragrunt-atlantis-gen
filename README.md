# terragrunt-atlantis-gen

Go wrapper around the Terragrunt CLI that produces the same `atlantis.yaml` as
[terragrunt-atlantis-config](https://github.com/ZeitOnline/terragrunt-atlantis-config)
(TAC) and replaces it as the Atlantis pre-workflow hook. Instead of importing
Terragrunt as a library, the wrapper asks the `terragrunt` binary — the same
one that runs the plans — for the structure via `terragrunt find --json`.

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

## Tests

```sh
go test ./...
```

`internal/goldens` builds the wrapper binary and replays every case through
the real CLI against its golden — byte for byte.

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

### 3. Copy settings to atlantis-projects.yaml

**Copy — don't move** — the six settings locals — `atlantis_workflow`,
`atlantis_terraform_version`, `atlantis_autoplan`, `atlantis_skip`,
`atlantis_apply_requirements`, `atlantis_project` — as rules into
`atlantis-projects.yaml` at the repo root:

- Parent configs (`root.hcl`, `env.hcl`, …, and `terragrunt.hcl` without
  `terraform.source`) → path `<dir>/**`, at the repo root `**`.
- Units → their exact directory path.
- Order: parents before units, shallow before deep directories. Later rules
  win per field — glob specificity thus reproduces the include chain.

```yaml
version: 1
overrides:
  - paths: ["**"]         # was: atlantis_workflow in root.hcl
    workflow: default
  - paths: ["prod/**"]    # was: atlantis_apply_requirements in prod/env.hcl
    apply_requirements: [approved, mergeable]
  - paths: ["prod/legacy-vpc"] # was: atlantis_skip = true in that unit
    skip: true
```

### 4. Cases that need a human decision

- **Computed `atlantis_*` locals** (function calls, references): express them
  as glob rules by hand — the wrapper does not evaluate HCL.
- **`terragrunt.hcl.json` units**: convert to HCL first; `terragrunt find`
  does not discover them.
- **Marker files** (`--project-hcl-files`): copy their
  `extra_atlantis_dependencies` as literal paths into the marker directory's
  rule — nothing includes the marker, so a mark inside it is never evaluated.
  `atlantis_project = true` becomes `project: true` in the rule.

### 5. Verify

Run TAC with the flags from the pre-workflow hook before and after the
migration; identical output proves the migration was additive:

```sh
terragrunt-atlantis-config generate --output before.yaml <hook flags>
# ... migrate ...
terragrunt-atlantis-config generate --output after.yaml <hook flags>
diff before.yaml after.yaml
```
