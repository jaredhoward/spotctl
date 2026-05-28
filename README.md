# spotctl

A Spotify Connect controller CLI, built in Go.

## Overview

`spotctl` controls Spotify playback via the Spotify Web API. While `spotctl` is a stand-alone application, it is designed to be called from automation tools like Home Assistant `shell_command` entries, enabling reliable cold-start playback on Spotify Connect devices without relying on third-party integrations.

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
make build-ha
```

## Setup

### 1. Run the interactive setup

```bash
spotctl setup --config ./config.yaml
```

This will prompt for:
- Spotify Client ID
- Spotify Client Secret
- Redirect URI (must match one configured in your Spotify app)

It will then walk through the OAuth flow and generate a `config.yaml` file.


```yaml
client_id: YOUR_CLIENT_ID
client_secret: YOUR_CLIENT_SECRET
refresh_token: YOUR_REFRESH_TOKEN
redirect_uri: https://your-redirect-uri
```

### 2. Add sets to `config.yaml`

Sets are named sequences of Spotify commands. A set can be as simple as a
single play command or as complex as a multi-step routine with confirmation
and error handling. Run a set with `spotctl run <name>`.

```yaml
sets:
  sleep_shuffle:
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
```

To find your device ID, use the `devices` command:

```bash
spotctl devices --config ./config.yaml
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
| `action` | *(required)* | `play`, `pause`, `next`, `previous`, `shuffle`, `volume`, `transfer`, `run_set` |
| `params` | — | Action-specific parameters (see below) |
| `confirm` | `false` | Poll Spotify state until the action is reflected before continuing |
| `timeout` | `15s` | Overall deadline for the command including confirmation polling |
| `on_error` | set-level or `continue` | `fail` \| `continue` \| `skip_remaining` |
| `on_timeout` | set-level or `continue` | `fail` \| `continue` \| `skip_remaining` |

#### Params reference

| Action | Required params | Optional params |
|---|---|---|
| `play` | — | `device_id`, `uri`, `playlist`, `track`, `album` |
| `pause` | — | `device_id` |
| `next` | — | `device_id` |
| `previous` | — | `device_id` |
| `shuffle` | — | `device_id`, `enabled` (default `true`) |
| `volume` | `level` | `device_id` |
| `transfer` | — | `device_id`, `play` (default `false`) |
| `run_set` | `set` | — |

For `play`, use one of `uri`, `playlist`, `track`, or `album` — not more than
one. `playlist`, `track`, and `album` are shorthand for the corresponding
`spotify:TYPE:ID` URI.

### 3. Discover and persist device names

Some Spotify Connect devices stop reporting a friendly name when they become
inactive. `spotctl` can persist device names so the `devices` output remains
readable even when Spotify omits the name.

```bash
spotctl devices --update --config ./config.yaml
```

This adds or updates a `device_names` mapping in `config.yaml`:

```yaml
device_names:
  a70e...: "Living Room Speaker"
  7b9a...: "Kitchen Echo"
```

### 4. Test

Run a set:
```bash
spotctl run sleep_shuffle --config ./config.yaml
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
spotctl volume --device DEVICE_ID --level 50 --config ./config.yaml
spotctl transfer --device DEVICE_ID --play --config ./config.yaml
```

## CLI Commands

<table>
  <thead>
    <tr><th>Command</th><th>Description</th></tr>
  </thead>
  <tbody>
    <tr><td><code>spotctl devices</code></td><td>List available Spotify Connect devices</td></tr>
    <tr><td><code>spotctl setup</code></td><td>Interactive setup and OAuth flow</td></tr>
    <tr><td><code>spotctl status</code></td><td>Show current Spotify playback status</td></tr>
    <tr><td><code>spotctl version</code></td><td>Print the version</td></tr>
    <tr><th colspan="2" align="center">Playback Commands</th></tr>
    <tr><td><code>spotctl run <set></code></td><td>Run a named set of commands</td></tr>
    <tr><td><code>spotctl play</code></td><td>Start or resume Spotify playback</td></tr>
    <tr><td><code>spotctl pause</code></td><td>Pause Spotify playback</td></tr>
    <tr><td><code>spotctl next</code></td><td>Skip to the next track</td></tr>
    <tr><td><code>spotctl previous</code></td><td>Return to the previous track</td></tr>
    <tr><td><code>spotctl transfer</code></td><td>Transfer playback to a Spotify Connect device</td></tr>
    <tr><td><code>spotctl volume</code></td><td>Set Spotify playback volume</td></tr>
  </tbody>
</table>

## Flags

### Global

| Flag | Default | Description |
|---|---|---|
| `--config` | `./config.yaml` | Path to config file |

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

### `transfer` vs. `play` without a URI

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

- Use `spotctl transfer --play` when handing off an active session from one device to another.
- Use `spotctl play --uri` (or a set) when starting something specific on a device.

## Re-authentication

When your refresh token expires, re-run setup:

```bash
spotctl setup --config ./config.yaml
```

Existing sets and device names will be preserved and credentials pre-filled for easy updating.

## Home Assistant Integration

### Deploy

Cross-compile and copy the binary to your HA instance:

```bash
make deploy
```

Copy `config.yaml` to `/config/scripts/config.yaml` on your HA instance.

### `shell_commands.yaml`

```yaml
spotctl_sleep: /config/scripts/spotctl run sleep_shuffle --config /config/scripts/config.yaml
```

### HA Script

When `confirm: true` is set on a play command in a set, `spotctl` blocks until
Spotify confirms playback has started before returning. This means the
`shell_command` step in HA completes only after music is actually playing —
no `wait_template` needed.

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
  - action: shell_command.spotctl_sleep
    # spotctl blocks here until Spotify confirms playback has started
    # (because confirm: true is set on the play command in the set).
    continue_on_error: false
  - action: media_player.volume_set
    target:
      entity_id: media_player.master_bedroom_speaker
    data:
      volume_level: 0.35
```

If you need HA to perform an action only after its own media player integration
reports the device as playing (e.g. for volume control via a native HA
integration rather than `spotctl`), you can still add a `wait_template` after
the `shell_command` step:

```yaml
  - wait_template: "{{ is_state('media_player.master_bedroom_speaker', 'playing') }}"
    timeout: "00:00:10"
    continue_on_timeout: true
```

The timeout can be short here — `spotctl` has already confirmed Spotify-side
playback, so HA's integration usually catches up within a second or two.

### Volume Control: `spotctl` vs. Home Assistant direct

When you set volume via `spotctl`, the signal chain is:

```
spotctl → Spotify API → Spotify Connect → device
```

This works well for devices that Home Assistant cannot control directly. When
HA already has a native integration for the device — Cast, Sonos, AirPlay, etc.
— letting HA own volume is usually the better choice:

- Fewer hops and lower latency (local network vs. cloud round-trip)
- Continues to work even if Spotify temporarily loses its connection to the device
- Volume is a device concern, not a playback concern

**Use `spotctl volume` when HA cannot reach the device directly.**
Otherwise, prefer HA's own volume action (shown in the script example above).

## Security

`config.yaml` contains sensitive credentials and is excluded from version
control via `.gitignore`. Never commit it to a public repository.

## Local Development

Copy `.env.example` to `.env` and fill in your HA details for deployment:

```bash
cp .env.example .env
```

`.env`:
```
HA_USER=root
HA_HOST=homeassistant.local
```
