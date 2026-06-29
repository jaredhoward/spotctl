package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "./config.yaml"

const DefaultPlaybackPollInterval = 500 * time.Millisecond

type Config struct {
	ClientID             string          `yaml:"client_id"`
	ClientSecret         string          `yaml:"client_secret"`
	RefreshToken         string          `yaml:"refresh_token"`
	RedirectURI          string          `yaml:"redirect_uri"`
	PlaybackPollInterval string          `yaml:"playback_poll_interval,omitempty"`
	Sets                 map[string]Set  `yaml:"sets,omitempty"`
	DeviceNames          map[string]string `yaml:"device_names"`
}

// PlaybackPollIntervalDuration returns the configured poll interval, falling
// back to DefaultPlaybackPollInterval if unset or unparseable.
func (c *Config) PlaybackPollIntervalDuration() time.Duration {
	if c.PlaybackPollInterval == "" {
		return DefaultPlaybackPollInterval
	}
	d, err := time.ParseDuration(c.PlaybackPollInterval)
	if err != nil || d <= 0 {
		return DefaultPlaybackPollInterval
	}
	return d
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Save(path string, cfg *Config) error {
	// Write to a sibling temp file and rename on success so a partial write
	// never destroys the existing config.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".spotctl-config-*.yaml")
	if err != nil {
		return fmt.Errorf("could not create temp config file: %w", err)
	}
	tmpName := tmp.Name()

	writeErr := func() error {
		enc := yaml.NewEncoder(tmp)
		if err := enc.Encode(cfg); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		return tmp.Close()
	}()

	if writeErr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("could not write config: %w", writeErr)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("could not save config file: %w", err)
	}
	return nil
}

func (c *Config) ClientB64() string {
	return base64.StdEncoding.EncodeToString(
		[]byte(c.ClientID + ":" + c.ClientSecret),
	)
}

func (c *Config) validate() error {
	missing := []string{}
	if c.ClientID == "" {
		missing = append(missing, "client_id")
	}
	if c.ClientSecret == "" {
		missing = append(missing, "client_secret")
	}
	if c.RefreshToken == "" {
		missing = append(missing, "refresh_token")
	}
	if len(missing) > 0 {
		return fmt.Errorf("config missing required fields: %v", missing)
	}
	return nil
}
