// Package config handles parsing and validation of comproc configuration files.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RestartPolicy defines when a service should be restarted.
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNever     RestartPolicy = "never"
)

// Service defines a single service configuration.
type Service struct {
	Name       string            `yaml:"-"`
	Command    string            `yaml:"command"`
	WorkingDir string            `yaml:"working_dir"`
	Env        map[string]string `yaml:"env"`
	Restart    RestartPolicy     `yaml:"restart"`
	DependsOn  []string          `yaml:"depends_on"`
}

// Config represents the entire comproc configuration.
type Config struct {
	Services     map[string]*Service `yaml:"services"`
	ServiceOrder []string            `yaml:"-"`
}

// ServiceNames returns service names in the order they appear in the config file.
func (c *Config) ServiceNames() []string {
	return c.ServiceOrder
}

// UnmarshalYAML implements custom unmarshaling to preserve service key order.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	// value is a mapping node with keys like "services"
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got %d", value.Kind)
	}

	// Find the "services" key and extract ordered keys
	for i := 0; i < len(value.Content)-1; i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		if keyNode.Value == "services" && valNode.Kind == yaml.MappingNode {
			for j := 0; j < len(valNode.Content)-1; j += 2 {
				c.ServiceOrder = append(c.ServiceOrder, valNode.Content[j].Value)
			}
			break
		}
	}

	// Use a temporary type to avoid infinite recursion
	type rawConfig Config
	var raw rawConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Services = raw.Services
	return nil
}

// Load reads and parses a configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return Parse(data)
}

// Parse parses configuration from YAML data.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set service names from map keys
	for name, svc := range cfg.Services {
		if svc == nil {
			cfg.Services[name] = &Service{Name: name}
		} else {
			svc.Name = name
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if len(c.Services) == 0 {
		return errors.New("no services defined")
	}

	for _, name := range c.ServiceOrder {
		if err := c.Services[name].Validate(c); err != nil {
			return fmt.Errorf("service %q: %w", name, err)
		}
	}

	// Check for circular dependencies
	if err := c.detectCycles(); err != nil {
		return err
	}

	return nil
}

// Validate checks a single service configuration.
func (s *Service) Validate(cfg *Config) error {
	if s.Command == "" {
		return errors.New("command is required")
	}

	// Validate restart policy
	switch s.Restart {
	case "", RestartNever, RestartOnFailure, RestartAlways:
		// Valid
	default:
		return fmt.Errorf("invalid restart policy: %q", s.Restart)
	}

	// Validate dependencies exist
	for _, dep := range s.DependsOn {
		if _, ok := cfg.Services[dep]; !ok {
			return fmt.Errorf("unknown dependency: %q", dep)
		}
	}

	return nil
}

// GetRestartPolicy returns the effective restart policy, defaulting to "never".
func (s *Service) GetRestartPolicy() RestartPolicy {
	if s.Restart == "" {
		return RestartNever
	}
	return s.Restart
}

// detectCycles checks for circular dependencies using DFS.
func (c *Config) detectCycles() error {
	// 0 = unvisited, 1 = in current path, 2 = fully visited
	state := make(map[string]int)

	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		if state[name] == 2 {
			return nil
		}
		if state[name] == 1 {
			// Found a cycle - find where it starts
			cycleStart := 0
			for i, n := range path {
				if n == name {
					cycleStart = i
					break
				}
			}
			cycle := append(path[cycleStart:], name)
			return fmt.Errorf("circular dependency detected: %v", cycle)
		}

		state[name] = 1
		path = append(path, name)

		svc := c.Services[name]
		for _, dep := range svc.DependsOn {
			if err := visit(dep, path); err != nil {
				return err
			}
		}

		state[name] = 2
		return nil
	}

	for _, name := range c.ServiceOrder {
		if err := visit(name, nil); err != nil {
			return err
		}
	}

	return nil
}

// TopologicalSort returns services in dependency order (dependencies first).
func (c *Config) TopologicalSort() ([]*Service, error) {
	var result []*Service
	visited := make(map[string]bool)

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		visited[name] = true

		svc := c.Services[name]
		for _, dep := range svc.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}

		result = append(result, svc)
		return nil
	}

	for _, name := range c.ServiceOrder {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return result, nil
}
