package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/thebug/lab/eko/v3/pkg/types"
)

const (
	ConfigDir     = ".config/eko"
	ConfigFile    = "config.json"
	DefaultModel      = "dolphin-phi"
	DefaultURL        = "http://localhost:11434"
	DefaultComfyUIURL = "http://localhost:8188"
	DefaultWorkflowPath = "~/lab/model/workflow/default.json"
)

// Config represents the application configuration
type Config struct {
	Model        string `json:"model"`
	URL          string `json:"url"`
	ComfyUIURL   string `json:"comfyui_url"`
	WorkflowPath string `json:"workflow_path"`
}

// Manager handles configuration operations
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		homeDir = "."
	}
	
	configPath := filepath.Join(homeDir, ConfigDir)
	return &Manager{
		configPath: configPath,
	}
}

// LoadConfig loads configuration from file
func (m *Manager) LoadConfig() tea.Cmd {
	return func() tea.Msg {
		// Ensure config directory exists
		if err := os.MkdirAll(m.configPath, 0755); err != nil {
			return types.ConfigLoadedMsg{ModelName: "", Err: err}
		}

		configFilePath := filepath.Join(m.configPath, ConfigFile)
		data, err := os.ReadFile(configFilePath)
		if err != nil {
			// If file doesn't exist, return default config
			if os.IsNotExist(err) {
				return types.ConfigLoadedMsg{ModelName: DefaultModel, URL: DefaultURL, Err: nil}
			}
			return types.ConfigLoadedMsg{ModelName: "", URL: "", Err: err}
		}

		var config Config
		if err := json.Unmarshal(data, &config); err != nil {
			return types.ConfigLoadedMsg{ModelName: "", URL: "", Err: err}
		}

		// Use default model if not specified
		if config.Model == "" {
			config.Model = DefaultModel
		}
		// Use default URL if not specified
		if config.URL == "" {
			config.URL = DefaultURL
		} else {
			// Add http:// protocol if missing
			if !strings.HasPrefix(config.URL, "http://") && !strings.HasPrefix(config.URL, "https://") {
				config.URL = "http://" + config.URL
			}
		}

		// Use default ComfyUI URL if not specified
		if config.ComfyUIURL == "" {
			config.ComfyUIURL = DefaultComfyUIURL
		} else {
			// Add http:// protocol if missing
			if !strings.HasPrefix(config.ComfyUIURL, "http://") && !strings.HasPrefix(config.ComfyUIURL, "https://") {
				config.ComfyUIURL = "http://" + config.ComfyUIURL
			}
		}

		// Use default workflow path if not specified
		if config.WorkflowPath == "" {
			config.WorkflowPath = DefaultWorkflowPath
		}

		return types.ConfigLoadedMsg{ModelName: config.Model, URL: config.URL, ComfyUIURL: config.ComfyUIURL, WorkflowPath: config.WorkflowPath, Err: nil}
	}
}

// SaveConfig saves configuration to file
func (m *Manager) SaveConfig(modelName string) tea.Cmd {
	return func() tea.Msg {
		// Ensure config directory exists
		if err := os.MkdirAll(m.configPath, 0755); err != nil {
			return nil
		}

		configFilePath := filepath.Join(m.configPath, ConfigFile)
		config := Config{Model: modelName}

		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return nil
		}

		if err := os.WriteFile(configFilePath, data, 0644); err != nil {
			return nil
		}

		return nil
	}
}

// CreateDummyConfig creates a dummy configuration file for testing
func (m *Manager) CreateDummyConfig() error {
	// Ensure config directory exists
	if err := os.MkdirAll(m.configPath, 0755); err != nil {
		return err
	}

	configFilePath := filepath.Join(m.configPath, ConfigFile)
	config := Config{Model: DefaultModel}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFilePath, data, 0644)
}
