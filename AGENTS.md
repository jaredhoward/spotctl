# spotctl — notes for AI assistants

## Settled architectural decisions

These have been explicitly decided. Do not re-propose alternatives.

**`DefaultConfigPath = "./config.yaml"` is intentionally relative.** Do not change it.

**`config.Save()` permission policy:** new files are created at `0600`; existing files preserve their current mode. This is intentional — do not cap or override existing permissions on save.

**`SpotifyClient` interface extraction was considered and declined.** The original pain point (global var mutation in tests) was resolved by `SetPlayerURL`. The remaining benefit does not justify changing `Action.Dispatch`'s signature.
