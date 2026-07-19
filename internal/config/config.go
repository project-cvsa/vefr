package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Listen       string   `toml:"listen"`
	AuthEnabled  *bool    `toml:"auth_enabled"`
	Username     string   `toml:"username"`
	Password     string   `toml:"password"`
	SourceIPs    []string `toml:"source_ips"`
	SourceCIDRs  []string `toml:"source_cidrs"`
	Rotation     string   `toml:"rotation"`
	AllowPorts   []int    `toml:"allow_ports"`
	BlockPrivate *bool    `toml:"block_private"`
	Timeouts     Timeouts `toml:"timeouts"`
}

type Timeouts struct {
	Connect     time.Duration `toml:"-"`
	ReadHeader  time.Duration `toml:"-"`
	Idle        time.Duration `toml:"-"`
	Request     time.Duration `toml:"-"`
	ConnectText string        `toml:"connect"`
	ReadText    string        `toml:"read_header"`
	IdleText    string        `toml:"idle"`
	RequestText string        `toml:"request"`
}

func (t *Timeouts) setDefaults() error {
	if t.ConnectText == "" {
		t.ConnectText = "15s"
	}
	if t.ReadText == "" {
		t.ReadText = "10s"
	}
	if t.IdleText == "" {
		t.IdleText = "90s"
	}
	if t.RequestText == "" {
		t.RequestText = "60s"
	}
	var err error
	if t.Connect, err = time.ParseDuration(t.ConnectText); err != nil {
		return fmt.Errorf("connect timeout: %w", err)
	}
	if t.ReadHeader, err = time.ParseDuration(t.ReadText); err != nil {
		return fmt.Errorf("read_header timeout: %w", err)
	}
	if t.Idle, err = time.ParseDuration(t.IdleText); err != nil {
		return fmt.Errorf("idle timeout: %w", err)
	}
	if t.Request, err = time.ParseDuration(t.RequestText); err != nil {
		return fmt.Errorf("request timeout: %w", err)
	}
	return nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse TOML: %w", err)
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8080"
	}
	if cfg.Rotation == "" {
		cfg.Rotation = "random"
	}
	if cfg.Rotation != "random" && cfg.Rotation != "round_robin" {
		return Config{}, errors.New("rotation must be random or round_robin")
	}
	if cfg.AuthEnabled == nil {
		authEnabled := true
		cfg.AuthEnabled = &authEnabled
	}
	if *cfg.AuthEnabled && (cfg.Username == "" || cfg.Password == "") {
		return Config{}, errors.New("username and password are required")
	}
	if len(cfg.SourceIPs) == 0 && len(cfg.SourceCIDRs) == 0 {
		return Config{}, errors.New("at least one source_ips or source_cidrs entry is required")
	}
	if len(cfg.AllowPorts) == 0 {
		cfg.AllowPorts = []int{80, 443}
	}
	for _, port := range cfg.AllowPorts {
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("invalid allow port %d", port)
		}
	}
	// Secure by default while still allowing an explicit development override.
	if cfg.BlockPrivate == nil {
		secure := true
		cfg.BlockPrivate = &secure
	}
	if err := cfg.Timeouts.setDefaults(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
