package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"

	"github.com/grpc-ecosystem/grpcdebug/cmd/verbose"
)

// SecurityType is the enum type of available security modes
type SecurityType int

const (
	// TypeInsecure is the insecure security mode and it is the default value
	TypeInsecure SecurityType = iota
	// TypeTls is the TLS security mode, which requires caller to provide
	// credentials to connect to peer
	TypeTls
)

// The environment variable name of getting the server configs
const grpcdebugServerConfigEnvName = "GRPCDEBUG_CONFIG"

func (e SecurityType) String() string {
	switch e {
	case TypeInsecure:
		return "Insecure"
	case TypeTls:
		return "TLS"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

// ServerConfig is the configuration for how to connect to a target
type ServerConfig struct {
	Pattern            string
	RealAddress        string
	Security           SecurityType
	CredentialFile     string
	ServerNameOverride string
}

func parseServerPattern(x string) (string, error) {
	var matcher = regexp.MustCompile(`^Server\s+?([A-Za-z0-9-_\.\*\?:]*)$`)
	tokens := matcher.FindStringSubmatch(x)
	if len(tokens) != 2 {
		return "", fmt.Errorf("Invalid server pattern: %v", x)
	}
	return strings.TrimSpace(tokens[1]), nil
}

func parseServerOption(x string) (string, string, error) {
	var matcher = regexp.MustCompile(`^(\w+?)\s+?(\S*)$`)
	tokens := matcher.FindStringSubmatch(x)
	if len(tokens) != 3 {
		return "", "", fmt.Errorf("Invalid server option: %v", x)
	}
	return strings.TrimSpace(tokens[1]), strings.TrimSpace(tokens[2]), nil
}

func loadServerConfigsFromFile(path string) []ServerConfig {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(bytes), "\n")
	var configs []ServerConfig
	var current *ServerConfig
	for i, line := range lines {
		if strings.HasPrefix(line, "Server") {
			pattern, err := parseServerPattern(line)
			if err != nil {
				log.Fatalf("Failed to parse config [%v:%d]: %v", path, i, err)
			}
			configs = append(configs, ServerConfig{Pattern: pattern})
			current = &configs[len(configs)-1]
		} else {
			stem := strings.TrimSpace(line)
			if stem == "" {
				// Allow black lines, skip them
				continue
			}
			key, value, err := parseServerOption(stem)
			if err != nil {
				log.Fatalf("Failed to parse config [%v:%d]: %v", path, i, err)
			}
			switch key {
			case "RealAddress":
				current.RealAddress = value
			case "Security":
				switch strings.ToLower(value) {
				case "insecure":
					current.Security = TypeInsecure
				case "tls":
					current.Security = TypeTls
				default:
					log.Fatalf("Unsupported security model: %v", value)
				}
			case "CredentialFile":
				current.CredentialFile = value
			case "ServerNameOverride":
				current.ServerNameOverride = value
			}
		}
	}
	verbose.Debugf("Loaded server configs from %v: %v", path, configs)
	return configs
}

func UserConfigDir() (string, error) {
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

func loadServerConfigs() []ServerConfig {
	if value := os.Getenv(grpcdebugServerConfigEnvName); value != "" {
		return loadServerConfigsFromFile(value)
	}
	// Try to load from work directory, if exists
	if _, err := os.Stat("./grpcdebug_config"); err == nil {
		return loadServerConfigsFromFile("./grpcdebug_config")
	}
	// Try to load from user config directory, if exists
	dir, _ := UserConfigDir()
	defaultUserConfig := path.Join(dir, "grpcdebug_config")
	if _, err := os.Stat(defaultUserConfig); err == nil {
		return loadServerConfigsFromFile(defaultUserConfig)
	}
	return nil
}

// GetServerConfig returns a connect configuration for the given target
func GetServerConfig(target string) ServerConfig {
	for _, config := range loadServerConfigs() {
		if config.Pattern == target {
			return config
		}
	}
	return ServerConfig{RealAddress: target}
}
