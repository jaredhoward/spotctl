# spotctl — notes for AI assistants

## Settled architectural decisions

These have been explicitly decided. Do not re-propose alternatives.

**`DefaultConfigPath = "./config.yaml"` is intentionally relative.** Do not change it.

**`config.Save()` permission policy:** new files are created at `0600`; existing files preserve their current mode. This is intentional — do not cap or override existing permissions on save.

**`SpotifyClient` interface extraction was considered and declined.** The original pain point (global var mutation in tests) was resolved by `SetPlayerURL`. The remaining benefit does not justify changing `Action.Dispatch`'s signature.

## Git workflow

`main` is protected: changes land via pull request, not direct pushes. CI (`.github/workflows/ci.yml`) must pass before a PR is mergeable — vet, build, test, a coverage-threshold gate, and `govulncheck`. There are no required human reviews (solo maintainer), so the gate is the CI check itself.

Tagging a release (`git tag vX.Y.Z && git push --tags`) triggers `.github/workflows/release.yml`, which builds the Home Assistant (`linux/arm64`) binary and publishes it as a GitHub Release asset. This replaces manually running `make build-ha-green` and uploading the binary by hand.

## Coverage expectations for AI-assisted changes

CI fails a PR if total coverage drops below the threshold set in `ci.yml` (`COVERAGE_THRESHOLD`, currently 85%; baseline as of 2026-07 is ~92%). Treat this as a floor, not a target — when adding or changing code, add or update tests in the same change rather than leaving coverage to a follow-up. Before calling a change complete, run `make coverage` (or `go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out`) and address any meaningful drop, the same way you'd address a failing test.
