# spotctl

A Spotify Connect controller CLI, built in Go.

## Overview

`spotctl` controls Spotify playback via the Spotify Web API. While `spotctl` is a stand-alone application, its original intention was to be called from automation tools like Home Assistant `shell_command` entries, enabling reliable playback on Spotify Connect devices.

## Requirements

- Go 1.26+
- A Spotify account
- A Spotify app created at [developer.spotify.com](https://developer.spotify.com/dashboard)

## Installation

### Build from source

```bash
git clone git@github.com:jaredhoward/spotctl.git
cd spotctl
go mod tidy
make build
```

### Cross-compile for Linux arm64 (e.g. Home Assistant Green)

```bash
make build-ha-green
```

### Development

After cloning, enable the pre-commit hook (vets, builds, and tests before every commit):

```bash
git config core.hooksPath .githooks
```

Run `make check` before opening a PR — it mirrors CI in full (vet, build, test, coverage gate, `govulncheck`).

## Setup

### 1. Run the interactive setup

```bash
spotctl setup --config ./config.yaml
```

This will prompt for your Spotify Client ID, Client Secret, and Redirect URI (which must match one configured in your Spotify app), then walk through the OAuth flow and generate a `config.yaml` file.

```yaml
client_id: YOUR_CLIENT_ID
client_secret: YOUR_CLIENT_SECRET
refresh_token: YOUR_REFRESH_TOKEN
redirect_uri: https://your-redirect-uri
```

An optional `playback_poll_interval` field controls how often `spotctl` polls Spotify to confirm state changes (default: `500ms`):

```yaml
playback_poll_interval: 500ms
```

An optional `confirm_stabilize_window` field controls how long a first confirmation is re-checked before `spotctl` trusts it (default: `4s`). This exists because some Spotify Connect devices — notably ones waking from an idle state — report a successful play and then silently drop a moment later; `spotctl` re-dispatches once if that happens within the window before giving up. Since that failure is specific to waking a device from idle, this window only applies when a device actually needed waking — that's `play` targeting a device (whether or not a URI is given), and `transfer --play` (see [`transfer` vs. `play` without a URI](#transfer-vs-play-without-a-uri) — it's the same operation under the hood). If the device is already the active Connect target, or the command has no wake step at all (`pause`, `next`, `previous`, `shuffle`, `repeat`, `volume`, or `transfer` without `--play`), a single confirmation is trusted immediately. A transient error from Spotify itself (502, 503, or 429 rate-limited) is retried out of that same one-retry budget regardless of the command — a 429 waits for the duration Spotify's `Retry-After` header specifies before retrying. This applies to both direct commands and to `run <set>`:

```yaml
confirm_stabilize_window: 4s
```

### 2. Discover and persist device IDs

Before configuring sets, find the device IDs of your Spotify Connect devices:

```bash
spotctl devices --config ./config.yaml
```

If a device isn't listed, Spotify may not be aware of it. Open the Spotify app and connect to the device from there first. Once Spotify recognizes it, it will appear in the list.

Some devices stop reporting a friendly name when inactive. Use `--update` to persist their names to `config.yaml` so the `devices` output stays readable:

```bash
spotctl devices --update --config ./config.yaml
```

This adds or updates a `device_names` mapping in `config.yaml`:

```yaml
device_names:
  a70e...: "Living Room Speaker"
  7b9a...: "Kitchen Echo"
```

## Commands

| Command | Description |
|---|---|
| `spotctl sets` | List all configured sets |
| `spotctl run <set>` | Run a named set of commands |
| `spotctl play` | Start or resume Spotify playback |
| `spotctl pause` | Pause Spotify playback |
| `spotctl next` | Skip to the next track |
| `spotctl previous` | Return to the previous track |
| `spotctl shuffle` | Enable or disable shuffle |
| `spotctl repeat` | Set repeat mode |
| `spotctl volume` | Set Spotify playback volume |
| `spotctl transfer` | Transfer playback to a Spotify Connect device |
| `spotctl devices` | List available Spotify Connect devices |
| `spotctl status` | Show current Spotify playback status |
| `spotctl recent` | Show recently played tracks |
| `spotctl setup` | Interactive setup and OAuth flow |
| `spotctl version` | Print the version |
| `spotctl call <path> [body]` | Call an arbitrary Spotify Web API endpoint directly, bypassing all of spotctl's action/confirm logic |

All playback commands (`play`, `pause`, `next`, `previous`, `shuffle`, `repeat`, `volume`, `transfer`) print the current playback status after the action completes, so you always know what the player is doing. Output is the same format as `spotctl status`.

### Examples

List configured sets:
```bash
spotctl sets --config ./config.yaml
```

Run a set:
```bash
spotctl run random_sleep --config ./config.yaml
```

One-off playback commands:
```bash
spotctl play --uri spotify:playlist:PLAYLIST_ID --config ./config.yaml
spotctl play --device DEVICE_ID --playlist PLAYLIST_ID --config ./config.yaml
spotctl play --device DEVICE_ID --track TRACK_ID --config ./config.yaml
spotctl play --device DEVICE_ID --album ALBUM_ID --config ./config.yaml
```

Playback control:
```bash
spotctl pause --config ./config.yaml
spotctl pause --device DEVICE_ID --config ./config.yaml
spotctl next --device DEVICE_ID --config ./config.yaml
spotctl previous --device DEVICE_ID --config ./config.yaml
spotctl shuffle --device DEVICE_ID --config ./config.yaml
spotctl shuffle --device DEVICE_ID --enabled=false --config ./config.yaml
spotctl repeat --device DEVICE_ID --state context --config ./config.yaml
spotctl repeat --device DEVICE_ID --state off --config ./config.yaml
spotctl volume --device DEVICE_ID --level 50 --config ./config.yaml
spotctl transfer --device DEVICE_ID --play --config ./config.yaml
```

Recently played tracks:
```bash
spotctl recent --config ./config.yaml
spotctl recent --limit 5 --config ./config.yaml
```

Raw API access (bypasses confirmation/polling entirely — useful for debugging, or hitting an endpoint spotctl doesn't wrap):
```bash
spotctl call /v1/me/player
spotctl call -X PUT '/v1/me/player/play?device_id=DEVICE_ID' '{"context_uri":"spotify:playlist:PLAYLIST_ID"}'
```

## Sets

Sets are optional — if you just want manual playback control, the commands above are all you need. A set is a named, reusable routine (e.g. "play this playlist, shuffle, set the volume") that can be as simple as a single play command or as complex as a multi-step routine with confirmation and error handling. Trigger one with `spotctl run <name>` or from an automation; list all configured sets with `spotctl sets`.

```yaml
sets:
  evening_playlist:
    device_id: DEVICE_ID
    commands:
      - action: play
        params:
          uri: spotify:playlist:PLAYLIST_ID
        timeout: 20s
        on_timeout: fail
      - action: shuffle
        params:
          enabled: true
      - action: repeat
        params:
          state: context
```

### Set-level fields

| Field | Default | Description |
|---|---|---|
| `device_id` | — | Applies to every command in the set. A command can override it with its own `device_id`. Omitting at both levels targets the currently active Spotify device. Accepts a `{{ name }}` placeholder resolved against the set's own declared `params` (literal values pass through unchanged) — see "Overriding the device at runtime" below. |
| `on_error` | `fail` | Default error policy for all commands in the set. |
| `on_timeout` | `fail` | Default timeout policy for all commands in the set. |
| `confirm` | `true` | Default confirmation setting for all commands in the set. Commands may override it. |
| `timeout` | `15s` | Default timeout for all commands in the set. Commands may override it. |
| `params` | — | Named parameters the set accepts. Each entry has `required: true/false` and/or a `default` — the default can be written as a bare scalar (`volume: 40`) or the full mapping form (`volume: { default: "40" }`). Callers supply values via `--<name>` flags on the CLI, or as sibling keys under `run_set` params. The reserved key `pool` (list of candidates, picked automatically — see below) always resolves to `uri`; `pool` and `uri` can't both be declared on the same set. |

### Set commands

Each command in a set has:

| Field | Default | Description |
|---|---|---|
| `action` | *(required)* | See actions table below |
| `name` | — | Optional label for this command. Used in log output and `spotctl sets` listings. |
| `device_id` | — | Spotify device ID for this command. Overrides the set-level `device_id`. Omit to target the active device. Also accepts a `{{ name }}` placeholder, same as the set-level field. |
| `params` | — | Action-specific parameters (see below) |
| `confirm` | set-level or `true` | Poll Spotify state until the action is reflected. For `play` (any device) or `transfer` with `play: true` on a device that needed waking from idle, also keeps re-checking for `confirm_stabilize_window` to make sure it holds (re-dispatching once if it drops) before continuing — other commands trust the first confirmation. Set to `false` to fire-and-forget. |
| `timeout` | set-level or `15s` | Overall deadline for the command including confirmation polling |
| `on_error` | set-level or `fail` | `fail` \| `continue` \| `skip_remaining` |
| `on_timeout` | set-level or `fail` | `fail` \| `continue` \| `skip_remaining` |

### Params reference

| Action | Params | Allowed values | Confirms by checking |
|---|---|---|---|
| `play` | one of `uri`, `playlist`, `track`, `album`, `artist` | one of these, or omit to resume on active device | `is_playing = true`, track URI changed if provided |
| `pause` | — | — | `is_playing = false` |
| `next` | — | — | track URI changed |
| `previous` | — | — | track URI changed |
| `shuffle` | `enabled` | `true` *(default)*, `false` | `shuffle_state = enabled` |
| `repeat` | `state` *(required)* | `off`, `track`, `context` | `repeat_state = state` |
| `volume` | `level` *(required)* | integer `0-100` | `device.volume_percent = level` |
| `transfer` | `play` | `true`, `false` *(default)* | `device.id = device_id`, plus `is_playing = true` when `play=true` |
| `sleep` | `duration` *(required)* | duration string like `30s` or `1m` | — |
| `run_set` | `set` *(required)*, plus declared target params | target set params depend on the inner set | inner set completes |

#### Action parameter details

- `play`: use exactly one of `uri`, `playlist`, `track`, `album`, or `artist`. The latter four are shorthand for `spotify:TYPE:ID` URIs.
- `transfer.play`: when `true`, confirmation waits for the target device to become active and playback to start.
- `sleep.duration`: no Spotify API call is made and `confirm` has no effect.
- `run_set.set`: target set name. Any other sibling params are passed to the inner set if declared there.

#### For `run_set`

Pass values to the target set's declared params as sibling keys alongside `set`:

```yaml
- action: run_set
  params:
    set: my_set
    uri: spotify:playlist:abc123
    volume: "50"
```

The target set must declare the corresponding params with `required: true` or a `default`.

To use a param value inside a command, use `{{ name }}` syntax:

```yaml
- action: play
  params:
    uri: '{{ uri }}'
```

`spotctl sets` renders these as `<name>` (e.g. `uri=<uri>`), indicating the value is supplied at call time. A concrete value (e.g. `level=35`) means a literal or default is set directly in the config.

#### Picking from a pool of values with `pool`

The reserved params key `pool` lists candidate entries to pick from on each run, resolved to `uri` — usable via `{{ uri }}` in commands or forwarded to a nested `run_set`, the same way any other param is. A set can't declare both `pool` and `uri`. An optional sibling `method` controls how the entry is picked; if omitted, it defaults to `random`.

##### `method: random` (default)

Picks uniformly at random on every run — no determinism, no file state written to `config.yaml`. Good for a pool you just want variety from, with no memory of past picks:

```yaml
sets:
  speaker_daily_mix:
    device_id: DEVICE_ID
    params:
      pool:
        - uri: spotify:playlist:AAAAAAAAAAAAAAAAAAAA
        - uri: spotify:playlist:BBBBBBBBBBBBBBBBBBBB
        - uri: spotify:playlist:CCCCCCCCCCCCCCCCCCCC
    commands:
      - action: play
        params:
          uri: '{{ uri }}'
        timeout: 20s
        on_timeout: fail
```

Omitting `method` entirely is equivalent to `method: random`.

##### `method: date`

Deterministically picks based on the current calendar date — no random seed or file state is involved. Good for a nightly set where you want variety without ever repeating last night's pick:

```yaml
sets:
  random_sleep:
    device_id: DEVICE_ID
    params:
      pool:
        - uri: spotify:playlist:AAAAAAAAAAAAAAAAAAAA
        - uri: spotify:playlist:BBBBBBBBBBBBBBBBBBBB
        - uri: spotify:playlist:CCCCCCCCCCCCCCCCCCCC
      method: date
    commands:
      - action: play
        params:
          uri: '{{ uri }}'
        timeout: 20s
        on_timeout: fail
      - action: shuffle
        params:
          enabled: true
      - action: repeat
        params:
          state: context
```

- Running the set again later the same day reproduces the same pick (safe to retry).
- The immediately preceding calendar day's pick is never repeated.
- Over `len(pool)` consecutive days, every pool entry appears exactly once.

`default` and `required` have no meaning on the reserved `pool` key — `spotctl` rejects config where either is set on it. `method`, if set, requires `pool` to also be set, and must be `random` or `date`.

##### Per-entry volume/shuffle/repeat overrides

A pool entry can override this set's own `volume`/`shuffle`/`repeat` params for that pick only, when a specific playlist needs different treatment than the rest of the pool (e.g. one track plays louder than the others, or shouldn't shuffle). Declare `volume`/`shuffle`/`repeat` as regular params (these become the fallback for entries that don't override them) and wire commands to them the same way as `uri`:

```yaml
sets:
  daily_mix:
    device_id: DEVICE_ID
    params:
      pool:
        - uri: spotify:playlist:AAAAAAAAAAAAAAAAAAAA
        - uri: spotify:playlist:BBBBBBBBBBBBBBBBBBBB
          volume: 25          # plays quieter than the rest of the pool
        - uri: spotify:playlist:CCCCCCCCCCCCCCCCCCCC
          shuffle: false       # played in order, unlike the rest of the pool
      method: date
      volume: 40
      shuffle: true
      repeat: off
    commands:
      - action: play
        params:
          uri: '{{ uri }}'
      - action: volume
        params:
          level: '{{ volume }}'
      - action: shuffle
        params:
          enabled: '{{ shuffle }}'
      - action: repeat
        params:
          state: '{{ repeat }}'
```

A pool entry that sets `volume`/`shuffle`/`repeat` requires the set to declare a matching `volume`/`shuffle`/`repeat` param (`spotctl` rejects config otherwise) — that param supplies the fallback value for every entry that doesn't override it. Precedence, highest to lowest: a caller-supplied `--volume`/`--shuffle`/`--repeat` (or `run_set` arg) always wins; then the picked entry's own override; then the set's declared `default`.

#### Overriding the device at runtime

`device_id` (on a set or a command) can reference a declared param with `{{ name }}`, exactly like `uri` or `level`. Declare the param and point `device_id` at it:

```yaml
sets:
  speaker_fade_in_play:
    device_id: '{{ device }}'
    params:
      device:
        default: 2cd72806a72944a01d1a70e77fb5de1f0b2a5ac8
      uri:
        required: true
    commands:
      - action: play
        params:
          uri: '{{ uri }}'
```

Since `device` is now a declared param, `--device` becomes a recognized flag automatically (the same mechanism that makes `--uri`/`--volume` work — see `spotctl run --help` above):

```bash
spotctl run speaker_fade_in_play --uri spotify:playlist:abc123 --device OTHER_DEVICE_ID
```

For a `run_set` command specifically, its own (resolved) `device_id` is automatically forwarded into the target set as a `device` arg — so a wrapper set only needs to expose its own `device` param and set its `run_set` command's `device_id` to `'{{ device }}'`; it doesn't need to also list `device` under that command's `params:`:

```yaml
sets:
  speaker_sleep:
    params:
      device: {}
      pool: [...]
    commands:
      - action: run_set
        device_id: '{{ device }}'
        params:
          set: speaker_fade_in_play
          uri: '{{ uri }}'
```

`spotctl run speaker_sleep --device X` then reaches `speaker_fade_in_play` without `speaker_fade_in_play` needing any changes beyond declaring its own `device` param. If the caller doesn't pass `--device` and nothing forwards one, each set's own `device_id`/default still applies — forwarding never overwrites a target set's default with an empty value. A set that never needs `--device` simply doesn't declare a `device` param at all.

## Flags

### Global

| Flag | Default | Description |
|---|---|---|
| `--config` | `./config.yaml` | Path to config file |
| `--verbose`, `-v` | `false` | Debug logging: dispatch/poll/confirm detail plus raw Spotify HTTP request/response tracing to stderr. Never logs credentials (access tokens, refresh tokens) — the OAuth token exchange is excluded entirely. Works on every command, including `run`. |

### `sets`

No additional flags. Reads config and prints each set name, command count, device, and a numbered list of its commands.

### `run`

| Flag | Description |
|---|---|
| `<set>` | Name of the set to run (required) |
| <code>-&#x2060;-&#x2060;&lt;param&gt;</code> | Value for a declared set param, e.g. `--uri=spotify:playlist:abc123`. Run `spotctl sets` to see declared params for each set. |

### `play`

| Flag | Description |
|---|---|
| `--device <id>` | Spotify device ID (omit to target active device) |
| `--uri <uri>` | Spotify context URI, e.g. `spotify:playlist:xxx` |
| `--playlist <id>` | Playlist ID — shorthand for `--uri spotify:playlist:ID` |
| `--track <id>` | Track ID — shorthand for `--uri spotify:track:ID` |
| `--album <id>` | Album ID — shorthand for `--uri spotify:album:ID` |
| `--artist <id>` | Artist ID — shorthand for `--uri spotify:artist:ID` |

Only one of `--uri`, `--playlist`, `--track`, `--album`, or `--artist` may be specified at a time. Each requires a non-empty value — passing an empty string (e.g. `--playlist "$ID"` when `$ID` is unset) is an error.

### `pause` / `next` / `previous`

| Flag | Description |
|---|---|
| `--device <id>` | Spotify device ID (omit to target active device) |

### `shuffle`

| Flag | Default | Description |
|---|---|---|
| `--device <id>` | — | Spotify device ID (omit to target active device) |
| `--enabled` | `true` | Enable shuffle; use `--enabled=false` to disable |

### `repeat`

| Flag | Default | Description |
|---|---|---|
| `--device <id>` | — | Spotify device ID (omit to target active device) |
| `--state` | `context` | Repeat mode: `off`, `track`, `context` |

### `volume`

| Flag | Description |
|---|---|
| `--device <id>` | Spotify device ID (omit to target active device) |
| `--level <0-100>` | Volume level |

### `transfer`

| Flag | Description |
|---|---|
| `--device <id>` | Spotify device ID to transfer playback to (required) |
| `--play` | Start playback immediately after transfer — see [`transfer` vs. `play` without a URI](#transfer-vs-play-without-a-uri) |

### `recent`

| Flag | Default | Description |
|---|---|---|
| `--limit <1-50>` | `20` | Number of recently played tracks to show |
| `--after <time>` | — | Only show tracks played after this time. Accepts RFC3339 (`2026-08-04T03:10:08-06:00`) or `2006-01-02 15:04:05`/`2006-01-02T15:04:05` (parsed in local time) |

Output is newest-first, same as Spotify returns it — so with `--after`, the *last* line printed is the first track played after that time.

### `call`

| Flag | Description |
|---|---|
| `<path>` | Required. Resolved against `https://api.spotify.com`, e.g. `/v1/me/player/play?device_id=xxx` |
| `[body]` | Optional raw request body, sent with `Content-Type: application/json` |
| `--method`, `-X` | HTTP method (default `GET`) |

Always prints `Status: <code>` plus the raw response body, even on a non-2xx response, so you can see exactly what the API said. Exits non-zero on a non-2xx status.

## `transfer` vs. `play` without a URI

```bash
# These two are the same operation:
spotctl transfer --device DEVICE_ID --play --config ./config.yaml
spotctl play --device DEVICE_ID --config ./config.yaml
```

`transfer --play` doesn't make its own API call — `Transfer` delegates directly to `Play` targeting the same device when `--play` is set, so the two commands run identical code. Both go through the same wake-then-resume sequence: if the device isn't already the active Connect target, `spotctl` first calls `PUT /me/player` with `play:false` (which is what actually migrates the session — queue, track position, shuffle/repeat all carry over), then resumes with a bodyless `PUT /me/player/play`. If the device is already active, both skip straight to the resume call.

Use `spotctl play --device DEVICE_ID` for this — it's the more general form, since it also accepts `--uri` (or a playlist/track/album/artist) when you want to start something specific rather than resume whatever the device last had loaded. `spotctl transfer --play` is kept as an alias with the same behavior.

`spotctl transfer` **without** `--play` is a genuinely different, simpler operation: it moves the Connect target to a device without confirming or starting playback — one raw API call, no wake logic, nothing to stabilize. Use it when you want to reposition a device without triggering playback.

## Re-authentication

When your refresh token expires, re-run setup:

```bash
spotctl setup --config ./config.yaml
```

Existing sets and device names will be preserved and credentials pre-filled for easy updating.

## Home Assistant Integration

### Shell Commands

Home Assistant's `shell_commands.yaml` integration lets you call external programs from automations. Here's how to define a shell command that runs a `spotctl` set:

```yaml
spotctl_random_sleep: /config/scripts/spotctl run random_sleep --config /config/scripts/config.yaml
```

This creates a callable action named `shell_command.spotctl_random_sleep` that Home Assistant can execute as part of automations and scripts.

### HA Script

When `confirm: true` is set on a play command in a set, `spotctl` blocks until Spotify confirms playback has started before returning. This means the `shell_command` step in HA completes only after music is actually playing — no `wait_template` needed.

```yaml
alias: Sleep Playlist
mode: restart
sequence:
  ...
  - action: shell_command.spotctl_random_sleep
    # spotctl blocks here until Spotify confirms playback has started
    # (because confirm: true is set on the play command in the set).
    continue_on_error: false
  ...
```

If you need HA to perform an action only after its own media player integration reports the device as playing (e.g. for volume control via a native HA integration rather than `spotctl`), you can add a short `wait_template` after the `shell_command` step:

```yaml
  - wait_template: "{{ is_state('media_player.master_bedroom_speaker', 'playing') }}"
    timeout: "00:00:10"
    continue_on_timeout: true
```

The timeout can be short — `spotctl` may have already confirmed Spotify-side playback, so HA's integration usually catches up within a second or two.

### Volume Control: `spotctl` vs. Home Assistant direct

When you set volume via `spotctl`, the signal chain is:

```
spotctl → Spotify API → Spotify Connect → device
```

This works well for devices that Home Assistant cannot control directly. When HA already has a native integration for the device — Cast, Sonos, AirPlay, etc. — letting HA own volume is usually the better choice: fewer hops, lower latency, and it continues to work even if Spotify temporarily loses its connection to the device.

**Use `spotctl volume` when HA cannot reach the device directly.** Otherwise, prefer HA's own volume action (shown in the script example above).

## Security

Your `config.yaml` will contain sensitive credentials and should be excluded from version control via `.gitignore`. Never commit it to a public repository.
