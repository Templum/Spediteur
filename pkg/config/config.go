package config

import (
	"errors"
	"io"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

type ForwardProxyConfig struct {
	Proxy      Proxy      `yaml:"Proxy"`
	Monitoring Monitoring `yaml:"Monitoring"`
}

type BufferSizes struct {
	Read  int `yaml:"Read"`
	Write int `yaml:"Write"`
}

type Limits struct {
	MaxBodySize   int `yaml:"MaxBodySize"`
	MaxConnsPerIP int `yaml:"MaxConnsPerIp"`
}

type Timeouts struct {
	Read    string `yaml:"Read"`
	Write   string `yaml:"Write"`
	Connect string `yaml:"Connect"`
}

type Proxy struct {
	Server      string      `yaml:"Server"`
	Port        uint16      `yaml:"Port"`
	BufferSizes BufferSizes `yaml:"BufferSizes"`
	Limits      Limits      `yaml:"Limits"`
	Timeouts    Timeouts    `yaml:"Timeouts"`
}

type Monitoring struct {
	Port uint16 `yaml:"Port"`
}

// New creates a config from the provided reader that should point towards a valid yaml version.
// During reading it will validate ports and timeouts.
func New(reader io.ReadCloser) (*ForwardProxyConfig, error) {
	defer reader.Close()
	var err error
	conf := ForwardProxyConfig{}

	rawYaml, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(rawYaml, &conf)
	if err != nil {
		return nil, err
	}

	err = validatePorts(conf)
	if err != nil {
		return nil, err
	}

	err = validateDurations(conf)
	if err != nil {
		return nil, err
	}

	fillDefaults(&conf)
	return &conf, nil
}

// validateDurations ensures that all off the provided durations are parseable
func validateDurations(conf ForwardProxyConfig) error {
	var err error

	_, err = time.ParseDuration(conf.Proxy.Timeouts.Read)
	if err != nil {
		return err
	}

	_, err = time.ParseDuration(conf.Proxy.Timeouts.Write)
	if err != nil {
		return err
	}

	_, err = time.ParseDuration(conf.Proxy.Timeouts.Connect)
	if err != nil {
		return err
	}

	return nil
}

// validatePorts ensures that the specified ports for the proxy and the monitoring service are within
// the valid range 1 < port < 65535.
func validatePorts(conf ForwardProxyConfig) error {
	validProxyPort := conf.Proxy.Port > 1 && conf.Proxy.Port < 65535
	if !validProxyPort {
		return errors.New("proxy port is not within valid range 1 < port < 65535")
	}

	validMonitoringPort := conf.Monitoring.Port > 1 && conf.Monitoring.Port < 65535
	if !validMonitoringPort {
		return errors.New("monitoring port is not within valid range 1 < port < 65535")
	}

	return nil
}

// fillDefaults ensures that the fields of the config that are unspecified by the user are set to defaults
func fillDefaults(conf *ForwardProxyConfig) {
	// First check for buffer sizes
	if conf.Proxy.BufferSizes.Read <= 0 {
		// This is the default value of fasthttp
		conf.Proxy.BufferSizes.Read = 4096
	}

	if conf.Proxy.BufferSizes.Write <= 0 {
		// This is the default value of fasthttp
		conf.Proxy.BufferSizes.Write = 4096
	}

	if conf.Proxy.Limits.MaxBodySize <= 0 {
		// This is the default value of fasthttp
		conf.Proxy.Limits.MaxBodySize = 4 * 1024 * 1024
	}
}
