package checks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// Config checks if the configuration file is valid
type Config struct{}

func (c *Config) Name() string {
	return i18n.T("doctor.check.config")
}

func (c *Config) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	// Get config file path
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = config.ConfigFilePath()
	}

	// Read the config file and the per-machine file next to it. They are
	// validated merged, as roji reads them: tunnel.domain in one and
	// tunnel.name in the other is a complete tunnel.
	data, files, err := config.ReadConfigFiles(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return doctor.CheckResult{
				Name:    c.Name(),
				Status:  doctor.Pass,
				Message: i18n.T("doctor.check.config.no_file"),
				Details: i18n.Tf("doctor.check.config.no_file_detail", configPath),
				Fixable: false,
			}
		}
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.config.read_failed"),
			Details: err.Error(),
			Fixable: false,
		}
	}

	// Run validation
	result := config.ValidateConfigYAML(data)
	read := strings.Join(files, ", ")

	if !result.HasIssues() {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.Tf("doctor.check.config.valid", read),
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
		summaryParts = append(summaryParts, i18n.Tf("doctor.check.config.unknown_keys", unknownKeyCount))
	}
	if invalidTypeCount > 0 {
		summaryParts = append(summaryParts, i18n.Tf("doctor.check.config.invalid_types", invalidTypeCount))
	}
	if missingRequiredCount > 0 {
		summaryParts = append(summaryParts, i18n.Tf("doctor.check.config.missing_required", missingRequiredCount))
	}
	if otherCount > 0 {
		summaryParts = append(summaryParts, i18n.Tf("doctor.check.config.other_issues", otherCount))
	}

	// Build details with all issues. The paths are into the merged document,
	// so say which files went into it.
	details := []string{i18n.Tf("doctor.check.config.read_files", read)}
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
		Message: i18n.Tf("doctor.check.config.issues_found", strings.Join(summaryParts, ", ")),
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
