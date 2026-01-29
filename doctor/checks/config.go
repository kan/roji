package checks

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
)

// Config checks if the configuration file is valid
type Config struct{}

func (c *Config) Name() string {
	return "Config file validation"
}

func (c *Config) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	// Get config file path
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = config.ConfigFilePath()
	}

	// Check if config file exists
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return doctor.CheckResult{
				Name:    c.Name(),
				Status:  doctor.Pass,
				Message: "No config file found (using defaults)",
				Details: fmt.Sprintf("Config file not found at %s", configPath),
				Fixable: false,
			}
		}
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: "Failed to read config file",
			Details: err.Error(),
			Fixable: false,
		}
	}

	// Run validation
	result := config.ValidateConfigYAML(data)

	if !result.HasIssues() {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: fmt.Sprintf("Config file is valid (%s)", configPath),
			Fixable: false,
		}
	}

	// Count issue types
	unknownKeyCount := 0
	invalidTypeCount := 0
	missingRequiredCount := 0
	otherCount := 0

	for _, issue := range result.Issues {
		switch issue.Type {
		case "unknown_key":
			unknownKeyCount++
		case "invalid_type":
			invalidTypeCount++
		case "missing_required":
			missingRequiredCount++
		default:
			otherCount++
		}
	}

	// Build summary message
	var summaryParts []string
	if unknownKeyCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d unknown key(s)", unknownKeyCount))
	}
	if invalidTypeCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d invalid type(s)", invalidTypeCount))
	}
	if missingRequiredCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d missing required field(s)", missingRequiredCount))
	}
	if otherCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d other issue(s)", otherCount))
	}

	// Build details with all issues
	var details []string
	for _, issue := range result.Issues {
		details = append(details, fmt.Sprintf("  - [%s] %s: %s", issue.Type, issue.Path, issue.Message))
	}

	// Determine status - unknown keys are warnings, type errors are failures
	status := doctor.Warn
	if invalidTypeCount > 0 || missingRequiredCount > 0 {
		status = doctor.Fail
	}

	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  status,
		Message: fmt.Sprintf("Config issues found: %s", strings.Join(summaryParts, ", ")),
		Details: strings.Join(details, "\n"),
		Fixable: false,
	}
}

func (c *Config) CanFix() bool {
	return false
}

func (c *Config) Fix(ctx context.Context, cfg *doctor.Config) error {
	return fmt.Errorf("config validation issues must be fixed manually")
}
