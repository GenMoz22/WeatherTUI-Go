package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Config represents persistent application settings and user data.
type Config struct {
	UseFahrenheit   bool     `json:"use_fahrenheit"`
	FavoriteCity    string   `json:"favorite_city"`
	RecentLocations []string `json:"recent_locations"`
}

// ConfigManager handles atomic thread-safe read and write operations for app configuration.
type ConfigManager struct {
	mu       sync.Mutex
	filePath string
}

// NewConfigManager initializes a ConfigManager resolving path ~/.local/share/WeatherTUI/config.json.
func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to locate user home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".local", "share", "WeatherTUI")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &ConfigManager{
		filePath: filepath.Join(configDir, "config.json"),
	}, nil
}

// Load reads configuration from disk or returns default configuration if file is missing.
func (cm *ConfigManager) Load() (Config, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	defaultCfg := Config{
		UseFahrenheit:   false,
		FavoriteCity:    "",
		RecentLocations: make([]string, 0),
	}

	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Info("Configuration file absent, loading defaults", "path", cm.filePath)
			return defaultCfg, nil
		}
		slog.Error("Failed to read config file", "path", cm.filePath, "error", err)
		return defaultCfg, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Error("Failed to unmarshal configuration JSON", "path", cm.filePath, "error", err)
		return defaultCfg, err
	}

	if cfg.RecentLocations == nil {
		cfg.RecentLocations = make([]string, 0)
	}

	slog.Info("Successfully loaded persistent configuration", "path", cm.filePath)
	return cfg, nil
}

// Save commits configuration updates to JSON storage atomically.
func (cm *ConfigManager) Save(cfg Config) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal configuration JSON", "error", err)
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmpFile := cm.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		slog.Error("Failed writing temporary config file", "path", tmpFile, "error", err)
		return fmt.Errorf("failed writing config file: %w", err)
	}

	if err := os.Rename(tmpFile, cm.filePath); err != nil {
		slog.Error("Failed replacing config file atomically", "error", err)
		return fmt.Errorf("failed persisting config file: %w", err)
	}

	slog.Debug("Configuration saved successfully", "path", cm.filePath)
	return nil
}

// UpdateRecentAndFavorite mutates recent locations and favorite status according to business constraints.
func UpdateRecentAndFavorite(recent []string, favorite string, newLoc string) ([]string, string) {
	cleanLoc := strings.TrimSpace(newLoc)
	if cleanLoc == "" {
		return recent, favorite
	}

	// Do not insert the target city into recent locations if it is already designated as favorite
	if strings.EqualFold(cleanLoc, favorite) {
		return recent, favorite
	}

	updated := make([]string, 0, len(recent)+1)
	for _, loc := range recent {
		if !strings.EqualFold(loc, cleanLoc) {
			updated = append(updated, loc)
		}
	}

	// Always prepend newly searched city to top of history
	updated = append([]string{cleanLoc}, updated...)

	// Cap search history to max 3 items
	if len(updated) > 3 {
		updated = updated[:3]
	}

	return updated, favorite
}
