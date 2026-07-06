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

### 3. Add sets to `config.yaml`

Sets are named sequences of Spotify commands. A set can be as simple as a single play command or as complex as a multi-step routine with confirmation and error handling. Run a set with `spotctl run <name>`, or list all configured sets with `spotctl sets`.

```yaml
sets:
  random_sleep:
    device_id: DEVICE_ID
    params:
      uri:
        pool:
          - spotify:playlist:PLAYLIST_ID_1
          - spotify:playlist:PLAYLIST_ID_2
          - spotify:playlist:PLAYLIST_ID_3
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

#### Set-level fields

| Field | Default | Description |
|---|---|---|
| `device_id` | — | Applies to every command in the set. A command can override it with its own `device_id`. Omitting at both levels targets the currently active Spotify device. |
| `on_error` | `fail` | Default error policy for all commands in the set. |
| `on_timeout` | `fail` | Default timeout policy for all commands in the set. |
| `confirm` | `true` | Default confirmation setting for all commands in the set. Commands may override it. |
| `timeout` | `15s` | Default timeout for all commands in the set. Commands may override it. |
| `params` | — | Named parameters the set accepts. Each entry has `required: true/false`, an optional `default`, or a `pool` (list of candidate values, picked automatically — see below). `pool` is mutually exclusive with `required`/`default`. Callers supply non-pool values via `--<name>` flags on the CLI, or as sibling keys under `run_set` params. |

#### Commands

Each command in a set has:

| Field | Default | Description |
|---|---|---|
| `action` | *(required)* | See actions table below |
| `name` | — | Optional label for this command. Used in log output and `spotctl sets` listings. |
| `device_id` | — | Spotify device ID for this command. Overrides the set-level `device_id`. Omit to target the active device. |
| `params` | — | Action-specific parameters (see below) |
| `confirm` | set-level or `true` | Poll Spotify state until the action is reflected before continuing. Set to `false` to fire-and-forget. |
| `timeout` | set-level or `15s` | Overall deadline for the command including confirmation polling |
| `on_error` | set-level or `fail` | `fail` \| `continue` \| `skip_remaining` |
| `on_timeout` | set-level or `fail` | `fail` \| `continue` \| `skip_remaining` |

#### Params reference

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

##### Action parameter details

- `play`: use exactly one of `uri`, `playlist`, `track`, `album`, or `artist`. The latter four are shorthand for `spotify:TYPE:ID` URIs.
- `transfer.play`: when `true`, confirmation waits for the target device to become active and playback to start.
- `sleep.duration`: no Spotify API call is made and `confirm` has no effect.
- `run_set.set`: target set name. Any other sibling params are passed to the inner set if declared there.

##### For `run_set`

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

##### Randomized picks with `pool`

A declared param can list a `pool` of candidate values instead of a fixed `default`:

```yaml
params:
  uri:
    pool:
      - spotify:playlist:AAAAAAAAAAAAAAAAAAAA
      - spotify:playlist:BBBBBBBBBBBBBBBBBBBB
      - spotify:playlist:CCCCCCCCCCCCCCCCCCCC
```

Each run picks one entry, resolved the same way as `default` (i.e. usable via `{{ uri }}` in commands or forwarded to a nested `run_set`). The pick is deterministic based on the current calendar date — no random seed or file state is involved, so nothing is written to `config.yaml`:

- Running the set again later the same day reproduces the same pick (safe to retry).
- The immediately preceding calendar day's pick is never repeated.
- Over `len(pool)` consecutive days, every pool entry appears exactly once.

`pool` is mutually exclusive with `default` and `required` on the same param — `spotctl` rejects config where both are set.

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

All playback commands (`play`, `pause`, `next`, `previous`, `shuffle`, `repeat`, `volume`, `transfer`) print the current playback status after the action completes, so you always know what the player is doing. Output is the same format as `spotctl status`.
| `spotctl setup` | Interactive setup and OAuth flow |
| `spotctl version` | Print the version |

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

## Flags

### Global

| Flag | Default | Description |
|---|---|---|
| `--config` | `./config.yaml` | Path to config file |

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
| `--play` | Start playback immediately after transfer |

## `transfer` vs. `play` without a URI

These two commands can look similar when no new context is intended:

```bash
# Transfer the active session to a device (session migration)
spotctl transfer --device DEVICE_ID --play --config ./config.yaml

# Resume playback on a device (no URI supplied)
spotctl play --device DEVICE_ID --config ./config.yaml
```

They map to different Spotify API calls and behave differently:

| | `transfer` | `play` (no URI) |
|---|---|---|
| API call | `PUT /me/player` | `PUT /me/player/play` |
| Carries over queue | Yes | No |
| Preserves track position | Yes | No |
| Preserves shuffle/repeat | Yes | No |
| Intended use | Moving an active session between devices | Resuming a device's last context |

Use `spotctl transfer --play` when handing off an active session from one device to another. Use `spotctl play --uri` (or a set) when starting something specific on a device.

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
