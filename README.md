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

The corpus has no cases for dropped features. Skipping stays gated by TAC's
own skip fixture (`skip`), which carries `exclude` blocks next to the
`atlantis_skip` locals so both binaries agree on it.

## Goldens

`testdata/goldens/` holds one golden per case in `internal/goldens/cases.go`:
the output the wrapper must produce, byte for byte. A case is a (fixture,
flags) pair named `<fixture>[_<flag variant>]`. 27 of the 39 goldens are the
frozen output of TAC **v2.25.1** (ZeitOnline fork, release binary,
checksum-verified), produced by real CLI invocations; they define "same as
TAC" for this project and only change deliberately.

The other twelve cases carry a `Diverges` annotation: their output is
set-equal but not byte-equal to TAC's (when_modified order — find reports
reads sorted — and files the units genuinely read via read_terragrunt_config,
which TAC never saw). Their golden is reviewed rather than frozen, and TAC's
own output for them lives in `testdata/goldens-tac/`.

`testdata/fixtures/` derives from TAC v2.25.1's `test/fixtures/` (MIT
license in `testdata/LICENSE-fixtures`), reduced to the wrapper's scope and
in the state a real repo is in when the wrapper runs: read marks, var-file
mirrors and `exclude` blocks, as described under "Migrating a repo". TAC
reads the tree unchanged — the marks are invisible to it — so one tree
serves both binaries.

Re-freezing (only ever deliberately, never as part of a normal test run):

```sh
go run ./tools/freeze -tac /path/to/terragrunt-atlantis-config   # TAC's output
go run ./tools/freeze -wrapper /path/to/terragrunt-atlantis-gen  # diverging cases
```

## Tests

```sh
go test ./...
```

`internal/goldens` builds the wrapper binary and replays every case through
the real CLI against its golden — byte for byte. Terragrunt ≥ 1.1 must be on
the PATH. `TestTACGoldens`, gated on `TAC_BIN` pointing at the frozen TAC
binary, replays TAC over the same fixtures against its goldens: the goldens
stay TAC's output rather than the wrapper's reading of it, and the marks in
the fixtures are proven invisible to TAC.

## Migrating a repo

One-off preparation of a Terragrunt repo for the wrapper, done by hand or in
an LLM session. The change is additive: everything TAC reads stays in place —
TAC and the wrapper run side by side on the same state. Cleaning up (removing
the locals) only happens after TAC has been retired.

### 1. Mark dependencies

Two modes; pick per repo:

**Direct replacement** — when the repo's pre-workflow hook switches to the
wrapper at the same time. The TAC locals are replaced by explicit read
marks; TAC loses these dependencies the moment the change merges, so merge
it together with the hook cutover for that repo. No machinery: marks are
plain list entries in a `locals` block, placed in whichever config owns the
knowledge — unit-specific marks in the unit itself (they may reference the
unit's other locals), subtree-shared marks in the shared included config,
whose functions evaluate per including child. This is the pattern in the
`iam` repo:

```hcl
locals {
  # Read-marks: terragrunt find reports these, which is how Atlantis
  # discovery and git-filtered runs know this unit depends on them.
  data_reads = [
    mark_glob_as_read("${get_repo_root()}/data/humans/**/*.yaml"),
  ]
}
```

Files a config already reads natively — `read_terragrunt_config()`,
`read_tfvars_file()`, `sops_decrypt_file()`, a local `terraform.source` —
appear in `reading` on their own and need no mark. `file()` does **not**
register a read (verified on Terragrunt v1.1.0); keep a mark for anything
only `file()` touches.

**Shadow-compatible** — when TAC keeps generating for the repo during a
side-by-side phase. Wrap every entry of `extra_atlantis_dependencies` in
place, so both tools read the same declaration:

- Entry without glob characters → `mark_as_read(<expression>)`. Function
  calls like `find_in_parent_folders(...)` and conditionals are wrapped as a
  whole.
- Entry whose literal text contains glob characters (`*`, `?`, `[`) →
  `try(mark_glob_as_read(<E>), <E>)`.
- Leave already-wrapped entries (`mark_as_read`, `mark_glob_as_read`, `try`)
  untouched.
- Fix non-string entries first: `mark_as_read` would coerce the value to a
  string and silently swallow TAC's error about it.

Traps, valid in both modes (all verified on Terragrunt v1.1.0 and TAC
v2.25.1):

- **Wrap marks in `try(…, fallback)` only while TAC still parses the repo**
  (shadow mode): TAC's embedded Terragrunt (v0.86.2) does not know
  `mark_glob_as_read` and aborts the whole generation without the guard.
  With a paired hook cutover the guard is unnecessary — plain marks, and
  Terragrunt ≥ 1.1 becomes the requirement for parsing the repo at all.
- **Mark every `**` glob with and without its `**` segment**: terragrunt's
  `**/` requires at least one directory, Atlantis's `when_modified` matching
  requires zero or more — files at the glob's top level would silently stop
  triggering plans otherwise.
- **Do not derive the variant with `replace(g, "/**/", "/")`**: a
  slash-wrapped pattern is a regular expression (and `**` an invalid one) in
  TAC's evaluator — use `split`/`join` as above.
- Marks only work in `locals` blocks — inside `inputs`, inside
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
