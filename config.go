package bunker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the runtime configuration loaded from config.yaml.
type Config struct {
	UI struct {
		SSHHost string `yaml:"ssh_host"`
		SSHPort string `yaml:"ssh_port"`
	} `yaml:"ui"`
	Server struct {
		Listen     string `yaml:"listen"`
		TrustProxy bool   `yaml:"trust_proxy"`
	} `yaml:"server"`
	SSHServer struct {
		Listen string `yaml:"listen"`
	} `yaml:"ssh_server"`
}

// scalarString accepts either a YAML string or a YAML number, so ssh_port can
// be written as "8022" or 8022.
type scalarString string

func (s *scalarString) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a string or number")
	}
	*s = scalarString(strings.TrimSpace(n.Value))
	return nil
}

// LoadConfig reads config.yaml from the data directory.
func LoadConfig(dir DataDir) (Config, error) {
	return loadConfigFile(filepath.Join(dir.String(), "config.yaml"))
}

func loadConfigFile(path string) (Config, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var raw struct {
		UI struct {
			SSHHost string       `yaml:"ssh_host"`
			SSHPort scalarString `yaml:"ssh_port"`
		} `yaml:"ui"`
		Server struct {
			Listen     string `yaml:"listen"`
			TrustProxy bool   `yaml:"trust_proxy"`
		} `yaml:"server"`
		SSHServer struct {
			Listen string `yaml:"listen"`
		} `yaml:"ssh_server"`
	}
	if err = yaml.Unmarshal(buf, &raw); err != nil {
		return Config{}, err
	}

	cfg := Config{}
	cfg.UI.SSHHost = strings.TrimSpace(raw.UI.SSHHost)
	cfg.UI.SSHPort = string(raw.UI.SSHPort)
	cfg.Server.Listen = strings.TrimSpace(raw.Server.Listen)
	cfg.Server.TrustProxy = raw.Server.TrustProxy
	cfg.SSHServer.Listen = strings.TrimSpace(raw.SSHServer.Listen)

	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8080"
	}
	if cfg.SSHServer.Listen == "" {
		cfg.SSHServer.Listen = ":8022"
	}
	return cfg, nil
}
