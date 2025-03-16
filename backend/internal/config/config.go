package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Environment  string `mapstructure:"environment"`
	Server       ServerConfig
	Database     DatabaseConfig
	Queue        QueueConfig
	Auth         AuthConfig
	Workers      WorkersConfig
	Integrations IntegrationsConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port                   int  `mapstructure:"port"`
	ReadTimeoutSeconds     int  `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds    int  `mapstructure:"write_timeout_seconds"`
	IdleTimeoutSeconds     int  `mapstructure:"idle_timeout_seconds"`
	ShutdownTimeoutSeconds int  `mapstructure:"shutdown_timeout_seconds"`
	EnableCORS             bool `mapstructure:"enable_cors"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
	FilePath string `mapstructure:"file_path"` // For SQLite
}

// QueueConfig holds job queue configuration
type QueueConfig struct {
	Driver   string `mapstructure:"driver"`
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret            string `mapstructure:"jwt_secret"`
	JWTExpirationMinutes int    `mapstructure:"jwt_expiration_minutes"`
	EncryptionKey        string `mapstructure:"encryption_key"`
}

// WorkersConfig holds worker configuration
type WorkersConfig struct {
	Count       int `mapstructure:"count"`
	MaxRetries  int `mapstructure:"max_retries"`
	RetryDelay  int `mapstructure:"retry_delay_seconds"`
	JobTimeout  int `mapstructure:"job_timeout_seconds"`
	PollTimeout int `mapstructure:"poll_timeout_seconds"`
}

// IntegrationsConfig holds configuration for external service integrations
type IntegrationsConfig struct {
	Jira       JiraConfig       `mapstructure:"jira"`
	Slack      SlackConfig      `mapstructure:"slack"`
	Teams      TeamsConfig      `mapstructure:"teams"`
	Google     GoogleConfig     `mapstructure:"google"`
	Microsoft  MicrosoftConfig  `mapstructure:"microsoft"`
	ServiceNow ServiceNowConfig `mapstructure:"servicenow"`
}

// JiraConfig holds Jira integration configuration
type JiraConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	RedirectURL  string `mapstructure:"redirect_url"`
	APIURL       string `mapstructure:"api_url"`
}

// SlackConfig holds Slack integration configuration
type SlackConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	RedirectURL  string `mapstructure:"redirect_url"`
	APIURL       string `mapstructure:"api_url"`
}

// TeamsConfig holds Microsoft Teams integration configuration
type TeamsConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	RedirectURL  string `mapstructure:"redirect_url"`
	APIURL       string `mapstructure:"api_url"`
}

// GoogleConfig holds Google Workspace integration configuration
type GoogleConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	RedirectURL  string `mapstructure:"redirect_url"`
	APIURL       string `mapstructure:"api_url"`
	Scopes       string `mapstructure:"scopes"`
}

// MicrosoftConfig holds Microsoft 365 integration configuration
type MicrosoftConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	RedirectURL  string `mapstructure:"redirect_url"`
	APIURL       string `mapstructure:"api_url"`
	Scopes       string `mapstructure:"scopes"`
}

// ServiceNowConfig holds ServiceNow integration configuration
type ServiceNowConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	AuthURL      string `mapstructure:"auth_url"`
	TokenURL     string `mapstructure:"token_url"`
	RedirectURL  string `mapstructure:"redirect_url"`
	APIURL       string `mapstructure:"api_url"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// Set default config values
	config := &Config{
		Environment: "development",
		Server: ServerConfig{
			Port:                   8080,
			ReadTimeoutSeconds:     30,
			WriteTimeoutSeconds:    30,
			IdleTimeoutSeconds:     60,
			ShutdownTimeoutSeconds: 15,
			EnableCORS:             true,
		},
		Database: DatabaseConfig{
			Driver:   "sqlite",
			FilePath: "auditcue.db",
		},
		Queue: QueueConfig{
			Driver:  "redis",
			Address: "localhost:6379",
			DB:      0,
		},
		Auth: AuthConfig{
			JWTSecret:            "change-me-in-production",
			JWTExpirationMinutes: 60,
			EncryptionKey:        "change-me-in-production-32-bytes-key",
		},
		Workers: WorkersConfig{
			Count:       5,
			MaxRetries:  3,
			RetryDelay:  60,
			JobTimeout:  300,
			PollTimeout: 5,
		},
		Integrations: IntegrationsConfig{
			Jira: JiraConfig{
				AuthURL:  "https://auth.atlassian.com/oauth/authorize",
				TokenURL: "https://auth.atlassian.com/oauth/token",
				APIURL:   "https://api.atlassian.com",
			},
			Slack: SlackConfig{
				AuthURL:  "https://slack.com/oauth/v2/authorize",
				TokenURL: "https://slack.com/api/oauth.v2.access",
				APIURL:   "https://slack.com/api",
			},
			Teams: TeamsConfig{
				AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				APIURL:   "https://graph.microsoft.com/v1.0",
			},
			Google: GoogleConfig{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
				APIURL:   "https://www.googleapis.com",
				Scopes:   "https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/drive",
			},
			Microsoft: MicrosoftConfig{
				AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				APIURL:   "https://graph.microsoft.com/v1.0",
				Scopes:   "User.Read Mail.Read Files.Read",
			},
			ServiceNow: ServiceNowConfig{
				AuthURL:  "https://{instance}.service-now.com/oauth_auth.do",
				TokenURL: "https://{instance}.service-now.com/oauth_token.do",
				APIURL:   "https://{instance}.service-now.com/api",
			},
		},
	}

	// Load configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/auditcue")

	// Read environment variables prefixed with AUDITCUE_
	viper.SetEnvPrefix("AUDITCUE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Find and read the config file if it exists
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, using defaults and environment variables
	}

	// Unmarshal config
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// validateConfig performs basic validation of the loaded configuration
func validateConfig(cfg *Config) error {
	// Create SQLite database directory if it doesn't exist
	if cfg.Database.Driver == "sqlite" {
		dbDir := filepath.Dir(cfg.Database.FilePath)
		if dbDir != "." && dbDir != "" {
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				return fmt.Errorf("failed to create database directory: %w", err)
			}
		}
	}

	return nil
}

// String returns a string representation of the config
func (c *Config) String() string {
	// Hide sensitive fields for display
	displayConfig := *c
	displayConfig.Auth.JWTSecret = "[REDACTED]"
	displayConfig.Auth.EncryptionKey = "[REDACTED]"
	displayConfig.Database.Password = "[REDACTED]"
	displayConfig.Queue.Password = "[REDACTED]"

	// Redact all client secrets
	displayConfig.Integrations.Jira.ClientSecret = "[REDACTED]"
	displayConfig.Integrations.Slack.ClientSecret = "[REDACTED]"
	displayConfig.Integrations.Teams.ClientSecret = "[REDACTED]"
	displayConfig.Integrations.Google.ClientSecret = "[REDACTED]"
	displayConfig.Integrations.Microsoft.ClientSecret = "[REDACTED]"
	displayConfig.Integrations.ServiceNow.ClientSecret = "[REDACTED]"

	b, _ := json.MarshalIndent(displayConfig, "", "  ")
	return string(b)
}
