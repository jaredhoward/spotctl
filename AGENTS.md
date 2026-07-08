# spotctl — notes for AI assistants

## Settled architectural decisions

These have been explicitly decided. Do not re-propose alternatives.

**`DefaultConfigPath = "./config.yaml"` is intentionally relative.** Do not change it.

**`config.Save()` permission policy:** new files are created at `0600`; existing files preserve their current mode. This is intentional — do not cap or override existing permissions on save.

**`SpotifyClient` interface extraction was considered and declined.** The original pain point (global var mutation in tests) was resolved by `SetPlayerURL`. The remaining benefit does not justify changing `Action.Dispatch`'s signature.

## Git workflow

`main` is protected: changes land via pull request, not direct pushes. CI (`.github/workflows/ci.yml`) must pass before a PR is mergeable — vet, build, test, a coverage-threshold gate, and `govulncheck`. There are no required human reviews (solo maintainer), so the gate is the CI check itself.

Tagging a release (`git tag vX.Y.Z && git push --tags`) triggers `.github/workflows/release.yml`, which builds the Home Assistant (`linux/arm64`) binary and publishes it as a GitHub Release asset. This replaces manually running `make build-ha-green` and uploading the binary by hand.

## Versioning policy

Strictly semver, even though `0.y.z` technically means "anything may change." Be deliberate about which segment moves:

- **Patch (`0.0.x`)**: bug fixes, internal cleanups, doc-only changes — nothing a `config.yaml` author needs to know about.
- **Minor (`0.x.0`)**: a new capability, or any change to existing behavior/defaults that a `config.yaml` author would need to know about (e.g. a new field, a changed default). Reset to patch bumps on top of it for fixes to that feature set.
- **`v1.0.0`**: reserved for a deliberate commitment that the config schema and CLI surface are stable — not a natural increment of whatever came before. `release.yml` marks every `v0.x` tag `--prerelease` automatically; that stops at `v1.0.0`.

When proposing a release, default to figuring out which category the accumulated changes since the last tag fall into rather than incrementing patch by habit.

## Coverage expectations for AI-assisted changes

CI fails a PR if total coverage drops below the threshold set in `ci.yml` (`COVERAGE_THRESHOLD`, currently 85%; baseline as of 2026-07 is ~92%). Treat this as a floor, not a target — when adding or changing code, add or update tests in the same change rather than leaving coverage to a follow-up. Before calling a change complete, run `make coverage` (or `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out`) and address any meaningful drop, the same way you'd address a failing test.
