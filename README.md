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

## Commands

| Command | Description |
|---|---|
| `spotctl setup` | Interactive setup and OAuth flow |
| `spotctl play` | Start Spotify playback (`run` is also supported as an alias) |
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