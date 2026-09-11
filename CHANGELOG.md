# Changelog

## [0.5.0](https://github.com/ZeitOnline/terragrunt-atlantis-gen/compare/v0.4.0...v0.5.0) (2026-09-11)


### Features

* structure the job log into sections ([6a20b60](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/6a20b60ac386922c0f28e222218970fab00d1256))


### Bug Fixes

* do not report ignored dependency blocks as watched paths ([64c29d5](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/64c29d50ad4752c0a4568c02bf6b29eba1b03180))
* keep the log's project tree and its total in agreement ([c561a04](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/c561a04208eccfdad9d4dd9245cc3b2e35b3b058))
* probe the terragrunt version where discovery runs ([e23d1c7](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/e23d1c760bc05dc10d0bd7daa254b55ca1cabab7))
* report base-state gains only for units that get a project ([246509d](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/246509d64988f712cd1eeb6533fa7d38d335fdf0))

## [0.4.0](https://github.com/ZeitOnline/terragrunt-atlantis-gen/compare/v0.3.0...v0.4.0) (2026-09-10)


### Features

* fail on suppressed parse errors by default ([1385eab](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/1385eab303b04c3f33adb0dedc0c53989dffcc31))
* fail on the parse errors terragrunt find suppresses ([2d8f259](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/2d8f25940909002a77870ea635fb19032fb1ff54))
* report the parse errors terragrunt find suppresses ([79a429d](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/79a429d46e9a3d13621df0d4ee049abad8d95566))


### Bug Fixes

* count suppressed parse errors only for the units --filter keeps ([9039654](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/90396546d21601cf994f4dd673728d27fbdc9809))
* count suppressed parse errors only for the units --filter keeps ([4905f70](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/4905f7001dd8285bdfd167d9b15310a06b595f28))
* match every wording terragrunt uses for a suppressed parse error ([1c20d02](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/1c20d021afb5960c05e6ec6aabd6fea1716cfbc0))
* report a root-level unit's parse error as "." instead of the absolute path ([95db72c](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/95db72cfa7d0f8f024b098b2f32d338a88ddb576))
* write the config to stdout when --output is not given ([73eb27d](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/73eb27d6fef36db782061d3db3488adbd562b80f))

## [0.3.0](https://github.com/ZeitOnline/terragrunt-atlantis-gen/compare/v0.2.0...v0.3.0) (2026-09-10)


### Features

* --base-ref watches what a unit read in the pull request's base ([0926df3](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/0926df3e5a836875601e88b66f88f5acdf990e3a))
* --base-ref watches what a unit read in the pull request's base ([657972f](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/657972f863ba6ebaba00c6603d605df6bf1616b3))


### Bug Fixes

* resolve --filter matches like the root, dedupe base merges across includes and reads ([a6b6077](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/a6b6077ad336853401dd80d856ad14ecfe81342b))
* treat a --root missing in the base as an empty base state ([5027c9a](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/5027c9ad317f7d67545ad9850f143c1d787a0d64))

## [0.2.0](https://github.com/ZeitOnline/terragrunt-atlantis-gen/compare/v0.1.0...v0.2.0) (2026-09-09)


### Features

* print a job log during generate ([5e709fd](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/5e709fd1544d0a01832a9ccf6ea15deb51fad5df))

## 0.1.0 (2026-09-08)


### Features

* add migrated fixture tree with TAC additivity gate ([5f73f5e](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/5f73f5e10a0f155664d532051a664f63ee11b29b))
* freeze TAC v2.25.1 behaviour as golden test corpus ([6932ce1](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/6932ce1173036032e42c2f86f8a3f203dfd69310))
* implement generate — parity suite green ([7855061](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/78550615459fe635a8a2220d3afb7d3520f4d473))
* narrow scope to terragrunt-native skip, drop settings locals ([65e1eef](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/65e1eefa04aed92eb6fb6c14aeacdf29ad003efa))


### Continuous Integration

* add GoReleaser and release-please release pipeline ([bc54445](https://github.com/ZeitOnline/terragrunt-atlantis-gen/commit/bc5444572a30ca9e98643882944e9d96a97333b5))

## Changelog
