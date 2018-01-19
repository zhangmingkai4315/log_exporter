package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

// Config define all the config labels
type Config struct {
	Global GlobalConfig `yaml:",omitempty"`
	Files  []FileConfig `yaml:",omitempty"`
}

// GlobalConfig define the global configurations, like server and groks
type GlobalConfig struct {
	Server      ServerConfig `yaml:",omitempty"`
	GrokDir     string       `yaml:"grok_dir,omitempty"`
	MetricsPath string       `yaml:"metrics_path,omitempty"`
}

func (gc *GlobalConfig) validateAndSetDefault() error {
	if gc.Server.Host == "" {
		gc.Server.Host = "0.0.0.0"
	}
	if gc.Server.Port == 0 {
		gc.Server.Port = 9898
	}
	if gc.MetricsPath == "" {
		gc.MetricsPath = "/metrics"
	}
	if gc.GrokDir == "" {
		return fmt.Errorf("grok dir not set")
	}
	if _, err := os.Stat(gc.GrokDir); os.IsNotExist(err) {
		return fmt.Errorf("grok dir not exist")
	}
	return nil
}

// ServerConfig only contain the basic server listen and host infomation
type ServerConfig struct {
	Host string `yaml:",omitempty"`
	Port int    `yaml:",omitempty"`
}

// ServerListenInfo will return url for listen http request
func (s *ServerConfig) ServerListenInfo() string {
	return s.Host + ":" + strconv.Itoa(s.Port)
}

// FileConfig define each files parse information
type FileConfig struct {
	Path        string       `yaml:",omitempty"`
	Readall     bool         `yaml:",omitempty"`
	Worker      int          `yaml:",omitempty"`
	Metric      MetricConfig `yaml:",omitempty"`
	Customgroks []string     `yaml:",omitempty"`
}

func (fc *FileConfig) validateAndSetDefault() error {
	if fc.Worker == 0 {
		fc.Worker = 1
	}
	if _, err := os.Stat(fc.Path); os.IsNotExist(err) {
		return fmt.Errorf("File: %v not exist", fc.Path)
	}
	if err := fc.Metric.validateAndSetDefault(); err != nil {
		return fmt.Errorf("metrics in file:%v valid error:%v", fc.Path, err)
	}
	return nil
}

// MetricConfig define metrics like type and name etc...
type MetricConfig struct {
	Type   string            `yaml:",omitempty"`
	Name   string            `yaml:",omitempty"`
	Help   string            `yaml:",omitempty"`
	Match  string            `yaml:",omitempty"`
	Labels map[string]string `yaml:",omitempty"`
}

func (mc *MetricConfig) validateAndSetDefault() error {
	switch {
	case mc.Type == "":
		return fmt.Errorf("'metrics.type' must not be empty")
	case mc.Name == "":
		return fmt.Errorf("'metrics.name' must not be empty")
	case mc.Help == "":
		return fmt.Errorf("'metrics.help' must not be empty")
	case mc.Match == "":
		return fmt.Errorf("'metrics.match' must not be empty")
	}
	return nil
}

// LoadConfig will load the config from file
func LoadConfig(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to load %v: %v", filename, err.Error())
	}

	cfg := &Config{}
	err = yaml.Unmarshal(content, cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal config file: %v", err.Error())
	}
	// do validation for some config object
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

//ValidateConfig Do validation work for config object
func ValidateConfig(cfg *Config) error {
	err := cfg.Global.validateAndSetDefault()
	if err != nil {
		return err
	}
	for _, file := range cfg.Files {
		if err := file.validateAndSetDefault(); err != nil {
			return err
		}
	}
	return nil
}
