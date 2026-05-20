# spotctl

A Spotify Connect controller CLI, built in Go.

## Overview

`spotctl` controls Spotify playback via the Spotify Web API. It is designed to be called from automation tools like Home Assistant `shell_command` entries, enabling reliable cold-start playback on Spotify Connect devices without relying on third-party integrations.

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
go build -o spotctl .
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

### 2. Add presets to `config.yaml`

Presets are optional but convenient for frequently used device and context combinations.

```yaml
client_id: YOUR_CLIENT_ID
client_secret: YOUR_CLIENT_SECRET
refresh_token: YOUR_REFRESH_TOKEN
redirect_uri: https://your-redirect-uri

presets:
  sleep:
    device_id: YOUR_SPOTIFY_DEVICE_ID
    context_uri: spotify:playlist:YOUR_PLAYLIST_ID
    shuffle: true
```

To find your device ID, use the `devices` command:

```bash
spotctl devices --config ./config.yaml
```

Device name persistence
-----------------------

Some Spotify Connect devices stop reporting a friendly `name` when they become inactive. `spotctl` can persist device names you observe so the `devices` output remains readable even when Spotify omits the name.

- To discover and save device names, run:

```bash
spotctl devices --update --config ./config.yaml
```

This will add (or update) a `device_names` mapping in your `config.yaml` like:

```yaml
device_names:
  2cd72806a72944a01d1a70e77fb5de1f0b2a5ac8: "Living Room Speaker"
  7b9a...: "Kitchen Echo"
```

After that, `spotctl devices` will use the stored name when the Spotify API returns an empty name for an inactive device.


### 3. Test

Using a preset:
```bash
spotctl play --preset sleep --config ./config.yaml
```

Using the full context URI directly:
```bash
spotctl play --device YOUR_DEVICE_ID --uri spotify:playlist:YOUR_PLAYLIST_ID --config ./config.yaml
```

Using convenience flags:
```bash
spotctl play --device YOUR_DEVICE_ID --playlist YOUR_PLAYLIST_ID --config ./config.yaml
spotctl play --device YOUR_DEVICE_ID --track YOUR_TRACK_ID --config ./config.yaml
spotctl play --device YOUR_DEVICE_ID --album YOUR_ALBUM_ID --config ./config.yaml
```

Flags override preset values when both are provided:
```bash
spotctl play --preset sleep --device OTHER_DEVICE_ID --config ./config.yaml
```

Playback control examples:
```bash
spotctl pause --device YOUR_DEVICE_ID --config ./config.yaml
spotctl next --device YOUR_DEVICE_ID --config ./config.yaml
spotctl previous --device YOUR_DEVICE_ID --config ./config.yaml
spotctl volume --device YOUR_DEVICE_ID --level 50 --config ./config.yaml
spotctl transfer --device YOUR_DEVICE_ID --play --config ./config.yaml
```

## Commands

| Command | Description |
|---|---|
| `spotctl setup` | Interactive setup and OAuth flow |
| `spotctl play` | Start Spotify playback (`run` is also supported as an alias) |
| `spotctl transfer` | Transfer playback to a Spotify Connect device |
| `spotctl pause` | Pause Spotify playback |
| `spotctl next` | Skip to the next track |
| `spotctl previous` | Return to the previous track |
| `spotctl volume` | Set Spotify playback volume |
| `spotctl devices` | List available Spotify Connect devices |
| `spotctl status` | Show current Spotify playback status |
| `spotctl version` | Print the version |

## Flags

### Global

| Flag | Default | Description |
|---|---|---|
| `--config` | `./config.yaml` | Path to config file |

### `run`

| Flag | Description |
|---|---|
| `--preset <name>` | Load a preset from config as a base |
| `--device <id>` | Spotify device ID (overrides preset) |
| `--uri <uri>` | Spotify context URI, e.g. `spotify:artist:xxx` (overrides preset) |
| `--playlist <id>` | Playlist ID, convenience for `--uri spotify:playlist:ID` |
| `--track <id>` | Track ID, convenience for `--uri spotify:track:ID` |
| `--album <id>` | Album ID, convenience for `--uri spotify:album:ID` |
| `--shuffle` | Enable shuffle (overrides preset) |

Only one of `--uri`, `--playlist`, `--track`, or `--album` may be specified at a time.

### `transfer` vs. `play` Without a URI

These two commands can look similar when no new track or context is intended:

```bash
# Transfer the active session to a device (session migration)
spotctl transfer --device YOUR_DEVICE_ID --play --config ./config.yaml

# Resume playback on a device (no URI supplied)
spotctl play --device YOUR_DEVICE_ID --config ./config.yaml
```

They map to different Spotify API calls and behave differently:

| | `transfer` | `play` (no URI) |
|---|---|---|
| API call | `PUT /me/player` | `PUT /me/player/play` |
| Carries over queue | Yes | No — uses the device's own last-known context |
| Preserves track position | Yes | No |
| Preserves shuffle/repeat | Yes | No |
| Intended use | Moving an active session between devices | Resuming whatever that device was last doing |

`transfer` is a session migration — Spotify treats it as the listener moving from one device to another mid-stream. `play` without a URI is a resume signal sent to a specific device, and its behaviour depends on whatever Spotify last remembers for that device, which can be inconsistent across device types.

- Use `spotctl transfer --play` when handing off an active session from one device to another.
- Use `spotctl play --uri` (or `--preset`) when starting something specific on a device.
- Avoid `spotctl play` with no URI and no preset — it is the least predictable path.

## Re-authentication

When your refresh token expires, re-run setup:

```bash
spotctl setup --config ./config.yaml
```

Existing presets will be preserved and credentials pre-filled for easy updating.

## Home Assistant Integration

### Deploy

Cross-compile and copy the binary to your HA instance:

```bash
make deploy
```

Copy `config.yaml` to `/config/scripts/config.yaml` on your HA instance.

### `shell_commands.yaml`

```yaml
spotify_sleep: /config/scripts/spotctl play --preset sleep --config /config/scripts/config.yaml
```

### HA Script

```yaml
alias: Sleep Playlist
mode: restart
sequence:
  - action: media_player.volume_set
    target:
      entity_id: media_player.master_bedroom_bed_speaker
    data:
      volume_level: 0
  - action: media_player.media_stop
    target:
      entity_id: media_player.master_bedroom_bed_speaker
    continue_on_error: true
  - action: shell_command.spotify_sleep
    continue_on_error: false
  - wait_template: "{{ is_state('media_player.master_bedroom_bed_speaker', 'playing') }}"
    timeout: "00:00:30"
    continue_on_timeout: true
  - action: media_player.volume_set
    target:
      entity_id: media_player.master_bedroom_bed_speaker
    data:
      volume_level: 0.35
```

### Volume Control: `spotctl` vs. Home Assistant Direct

When you set volume via `spotctl`, the signal chain is:

```
spotctl → Spotify API → Spotify Connect → device
```

This works well for standalone use and for devices that Home Assistant cannot control directly (a phone, a laptop, or a third-party Spotify Connect receiver with no HA integration). However, when HA already has a native integration for the device — Cast, Sonos, AirPlay, etc. — letting HA own volume is the better choice:

- Fewer hops and lower latency (local network vs. cloud round-trip)
- Continues to work even if Spotify temporarily loses its connection to the device
- Volume is a device concern, not a playback concern; keeping it with HA is the cleaner model
- Avoids unintentionally affecting non-Spotify audio sources on the same device

The exception is when you specifically want volume tied to the Spotify session — for example, to interact correctly with Spotify's normalization or cross-fade — or when HA simply has no path to the device.

**Use `spotctl volume` when HA cannot reach the device directly:**

```bash
spotctl volume --device YOUR_DEVICE_ID --level 50 --config /config/scripts/config.yaml
```

**Otherwise, prefer HA's own volume action** (shown in the HA Script example above).

## Security

`config.yaml` contains sensitive credentials and is excluded from version control via `.gitignore`. Never commit it to a public repository.

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