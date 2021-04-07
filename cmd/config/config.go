package config

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"runtime"

	"gopkg.in/yaml.v2"

	"github.com/grpc-ecosystem/grpcdebug/cmd/verbose"
)

// SecurityType is the enum type of available security modes
type SecurityType string

const (
	// TypeInsecure is the insecure security mode and it is the default value
	TypeInsecure SecurityType = "insecure"
	// TypeTls is the TLS security mode, which requires caller to provide
	// credentials to connect to peer
	TypeTls = "tls"
)

// The environment variable name of getting the server configs
const grpcdebugServerConfigEnvName = "GRPCDEBUG_CONFIG"

// ServerConfig is the configuration for how to connect to a target
type ServerConfig struct {
	RealAddress        string
	Security           SecurityType
	CredentialFile     string
	ServerNameOverride string
}

type grpcdebugConfig struct {
	Servers map[string]ServerConfig
}

func loadServerConfigsFromFile(path string) map[string]ServerConfig {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	var config grpcdebugConfig
	err = yaml.Unmarshal(bytes, config)
	if err != nil {
		panic(err)
	}
	verbose.Debugf("Loaded grpcdebug config from %v: %v", path, config)
	return config.Servers
}

// userConfigDir is copied here, so we can support Go v1.12
func userConfigDir() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "windows":
		dir = os.Getenv("AppData")
		if dir == "" {
			return "", errors.New("%AppData% is not defined")
		}

	case "darwin", "ios":
		dir = os.Getenv("HOME")
		if dir == "" {
			return "", errors.New("$HOME is not defined")
		}
		dir += "/Library/Application Support"

	case "plan9":
		dir = os.Getenv("home")
		if dir == "" {
			return "", errors.New("$home is not defined")
		}
		dir += "/lib"

	default: // Unix
		dir = os.Getenv("XDG_CONFIG_HOME")
		if dir == "" {
			dir = os.Getenv("HOME")
			if dir == "" {
				return "", errors.New("neither $XDG_CONFIG_HOME nor $HOME are defined")
			}
			dir += "/.config"
		}
	}
	return dir, nil
}

func loadServerConfigs() map[string]ServerConfig {
	if value := os.Getenv(grpcdebugServerConfigEnvName); value != "" {
		return loadServerConfigsFromFile(value)
	}
	// Try to load from work directory, if exists
	if _, err := os.Stat("./grpcdebug_config.yaml"); err == nil {
		return loadServerConfigsFromFile("./grpcdebug_config.yaml")
	}
	// Try to load from user config directory, if exists
	dir, _ := userConfigDir()
	defaultUserConfig := path.Join(dir, "grpcdebug_config.yaml")
	if _, err := os.Stat(defaultUserConfig); err == nil {
		return loadServerConfigsFromFile(defaultUserConfig)
	}
	return nil
}

// GetServerConfig returns a connect configuration for the given target
func GetServerConfig(target string) ServerConfig {
	for pattern, config := range loadServerConfigs() {
		// TODO(lidiz): support wildcards
		if pattern == target {
			if config.RealAddress == "" {
				config.RealAddress = pattern
			}
			return config
		}
	}
	return ServerConfig{RealAddress: target}
}
