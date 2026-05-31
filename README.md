# spotctl

A Spotify Connect controller CLI, built in Go.

## Overview

`spotctl` controls Spotify playback via the Spotify Web API. While `spotctl` is a stand-alone application, its original intention was to be called from automation tools like Home Assistant `shell_command` entries, enabling reliable playback on Spotify Connect devices.

## Requirements

- Go 1.21+
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

### Cross-compile for Linux x86_64 (e.g. Home Assistant Green)

```bash
make build-ha-green
```

## Setup

### 1. Run the interactive setup

```bash
spotctl setup --config ./config.yaml
```

This will prompt for your Spotify Client ID, Client Secret, and Redirect URI (which must match one configured in your Spotify app), then walk through the OAuth flow and generate a `config.yaml` file.

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
client_id: YOUR_CLIENT_ID
client_secret: YOUR_CLIENT_SECRET
refresh_token: YOUR_REFRESH_TOKEN
redirect_uri: https://your-redirect-uri

sets:
  random_sleep:
    device_id: DEVICE_ID
    commands:
      - action: play
        params:
          uri: spotify:playlist:PLAYLIST_ID
        confirm: true
        timeout: 20s
        on_timeout: fail
      - action: shuffle
        params:
          enabled: true
        confirm: true
        timeout: 5s
      - action: repeat
        params:
          repeat_state: context
        confirm: true
        timeout: 5s
```

#### Set-level fields

| Field | Default | Description |
|---|---|---|
| `device_id` | — | Applies to every command in the set. A command can override it with its own `params.device_id`. Omitting at both levels targets the currently active Spotify device. |
| `on_error` | `continue` | Default error policy for all commands in the set. |
| `on_timeout` | `continue` | Default timeout policy for all commands in the set. |

#### Commands

Each command in a set has:

| Field | Default | Description |
|---|---|---|
| `action` | *(required)* | See actions table below |
| `params` | — | Action-specific parameters (see below) |
| `confirm` | `false` | Poll Spotify state until the action is reflected before continuing |
| `timeout` | `15s` | Overall deadline for the command including confirmation polling |
| `on_error` | set-level or `continue` | `fail` \| `continue` \| `skip_remaining` |
| `on_timeout` | set-level or `continue` | `fail` \| `continue` \| `skip_remaining` |

#### Params reference

| Action | Required params | Optional params | Confirms by checking |
|---|---|---|---|
| `play` | — | `device_id`, `uri`, `playlist`, `track`, `album` | `is_playing = true` |
| `pause` | — | `device_id` | `is_playing = false` |
| `next` | — | `device_id` | track URI changed |
| `previous` | — | `device_id` | track URI changed |
| `shuffle` | — | `device_id`, `enabled` (default `true`) | `shuffle_state = enabled` |
| `repeat` | `repeat_state` | `device_id` | `repeat_state` matches |
| `volume` | `level` | `device_id` | `device.volume_percent = level` |
| `transfer` | — | `device_id`, `play` (default `false`) | `device.id = device_id` |
| `run_set` | `set` | — | inner set completes |
| `sleep` | `duration` | — | — |

Notes:

- For `play`, use one of `uri`, `playlist`, `track`, or `album` — not more than one. `playlist`, `track`, and `album` are shorthand for the corresponding `spotify:TYPE:ID` URI.
- `repeat_state` must be one of `off`, `track`, or `context`.
- `sleep` pauses execution for the specified duration (e.g. `30s`, `1m`). No Spotify API call is made. `confirm` has no effect.

### 4. Test

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
| `spotctl setup` | Interactive setup and OAuth flow |
| `spotctl version` | Print the version |

## Flags

### Global

| Flag | Default | Description |
|---|---|---|
| `--config` | `./config.yaml` | Path to config file |

### `sets`

No additional flags. Reads config and prints each set name, command count, and device.

### `run`

| Flag | Description |
|---|---|
| `<set>` | Name of the set to run (required) |

### `play`

| Flag | Description |
|---|---|
| `--device <id>` | Spotify device ID (omit to target active device) |
| `--uri <uri>` | Spotify context URI, e.g. `spotify:playlist:xxx` |
| `--playlist <id>` | Playlist ID — shorthand for `--uri spotify:playlist:ID` |
| `--track <id>` | Track ID — shorthand for `--uri spotify:track:ID` |
| `--album <id>` | Album ID — shorthand for `--uri spotify:album:ID` |

Only one of `--uri`, `--playlist`, `--track`, or `--album` may be specified at a time.

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

### `shell_commands.yaml`

```yaml
spotctl_random_sleep: /config/scripts/spotctl run random_sleep --config /config/scripts/config.yaml
```

### HA Script

When `confirm: true` is set on a play command in a set, `spotctl` blocks until Spotify confirms playback has started before returning. This means the `shell_command` step in HA completes only after music is actually playing — no `wait_template` needed.

```yaml
alias: Sleep Playlist
mode: restart
sequence:
  - action: media_player.volume_set
    target:
      entity_id: media_player.master_bedroom_speaker
    data:
      volume_level: 0
  - action: media_player.media_stop
    target:
      entity_id: media_player.master_bedroom_speaker
    continue_on_error: true
  - action: shell_command.spotctl_random_sleep
    # spotctl blocks here until Spotify confirms playback has started
    # (because confirm: true is set on the play command in the set).
    continue_on_error: false
  - action: media_player.volume_set
    target:
      entity_id: media_player.master_bedroom_speaker
    data:
      volume_level: 0.35
```

If you need HA to perform an action only after its own media player integration reports the device as playing (e.g. for volume control via a native HA integration rather than `spotctl`), you can add a short `wait_template` after the `shell_command` step:

```yaml
  - wait_template: "{{ is_state('media_player.master_bedroom_speaker', 'playing') }}"
    timeout: "00:00:10"
    continue_on_timeout: true
```

The timeout can be short — `spotctl` has already confirmed Spotify-side playback, so HA's integration usually catches up within a second or two.

### Volume Control: `spotctl` vs. Home Assistant direct

When you set volume via `spotctl`, the signal chain is:

```
spotctl → Spotify API → Spotify Connect → device
```

This works well for devices that Home Assistant cannot control directly. When HA already has a native integration for the device — Cast, Sonos, AirPlay, etc. — letting HA own volume is usually the better choice: fewer hops, lower latency, and it continues to work even if Spotify temporarily loses its connection to the device.

**Use `spotctl volume` when HA cannot reach the device directly.** Otherwise, prefer HA's own volume action (shown in the script example above).

## Security

Your `config.yaml` will contain sensitive credentials and should be excluded from version control via `.gitignore`. Never commit it to a public repository.
