package promptsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   int               `yaml:"version"`
	Source    string            `yaml:"source"`
	Variables map[string]string `yaml:"variables"`
	Targets   []Target          `yaml:"targets"`
}

type Target struct {
	Path     string `yaml:"path"`
	Template string `yaml:"template"`
}

func loadConfig(path string) (Config, string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", fmt.Errorf("read config %q: %w", path, err)
	}

	var config Config
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return Config{}, "", fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := config.validate(); err != nil {
		return Config{}, "", err
	}

	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, "", fmt.Errorf("resolve config directory: %w", err)
	}
	return config, root, nil
}

func (config Config) validate() error {
	if config.Version != 1 {
		return fmt.Errorf("unsupported config version %d; expected 1", config.Version)
	}
	if strings.TrimSpace(config.Source) == "" {
		return fmt.Errorf("config source must not be empty")
	}
	if len(config.Targets) == 0 {
		return fmt.Errorf("config must define at least one target")
	}

	seen := make(map[string]struct{}, len(config.Targets))
	for index, target := range config.Targets {
		if strings.TrimSpace(target.Path) == "" {
			return fmt.Errorf("target %d path must not be empty", index+1)
		}
		key := filepath.Clean(target.Path)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate target path %q", target.Path)
		}
		seen[key] = struct{}{}
	}
	return nil
}
