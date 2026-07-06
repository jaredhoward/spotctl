package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	if info, err := os.Stat(path); err == nil {
		if perm := info.Mode().Perm(); perm&0077 != 0 {
			fmt.Fprintf(os.Stderr, "warning: config file %s has permissions %04o — credentials may be readable by other users\n", path, perm)
		}
	}

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
	// Capture the existing file's permissions before writing; fall back to
	// 0600 for new files. Applied after the rename so the temp file never
	// carries more permissive bits than its own 0600 default while it's
	// visible under its temp name.
	perm := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	// Write to a sibling temp file and rename on success so a partial write
	// never destroys the existing config.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".spotctl-config-*.yaml")
	if err != nil {
		return fmt.Errorf("could not create temp config file: %w", err)
	}
	tmpName := tmp.Name()

	// Track whether tmp was closed inside the write closure to avoid a
	// double-close in the error path. Only set after Close() succeeds, so a
	// failed Close still falls through to the cleanup close below.
	tmpClosed := false
	writeErr := func() error {
		enc := yaml.NewEncoder(tmp)
		if err := enc.Encode(cfg); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		if err := tmp.Close(); err != nil {
			return err
		}
		tmpClosed = true
		return nil
	}()

	if writeErr != nil {
		if !tmpClosed {
			tmp.Close()
		}
		os.Remove(tmpName)
		return fmt.Errorf("could not write config: %w", writeErr)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("could not save config file: %w", err)
	}

	if perm != 0600 {
		if err := os.Chmod(path, perm); err != nil {
			return fmt.Errorf("could not set config file permissions: %w", err)
		}
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
	for name, set := range c.Sets {
		if err := validateOnFailure(set.OnError, "sets."+name+".on_error"); err != nil {
			return err
		}
		if err := validateOnFailure(set.OnTimeout, "sets."+name+".on_timeout"); err != nil {
			return err
		}
		paramNames := make([]string, 0, len(set.Params))
		for pname := range set.Params {
			paramNames = append(paramNames, pname)
		}
		sort.Strings(paramNames)
		for _, pname := range paramNames {
			decl := set.Params[pname]
			if len(decl.Pool) > 0 && decl.Default != "" {
				return fmt.Errorf("config field sets.%s.params.%s: pool and default are mutually exclusive", name, pname)
			}
			if len(decl.Pool) > 0 && decl.Required {
				return fmt.Errorf("config field sets.%s.params.%s: pool and required are mutually exclusive (pool always yields a value)", name, pname)
			}
		}
		for i, cmd := range set.Commands {
			loc := fmt.Sprintf("sets.%s.commands[%d]", name, i)
			if err := validateOnFailure(cmd.OnError, loc+".on_error"); err != nil {
				return err
			}
			if err := validateOnFailure(cmd.OnTimeout, loc+".on_timeout"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateOnFailure(v OnFailure, field string) error {
	switch v {
	case "", OnFailureFail, OnFailureContinue, OnFailureSkipRemaining:
		return nil
	default:
		return fmt.Errorf("config field %s has invalid value %q (must be fail, continue, or skip_remaining)", field, v)
	}
}
