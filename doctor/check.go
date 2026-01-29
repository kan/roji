// Package doctor provides environment diagnostics for roji.
//
// It checks the system environment for common issues and can auto-fix
// some problems when requested.
package doctor

import (
	"context"

	"github.com/kan/roji/config"
)

// Status represents the result status of a check
type Status int

const (
	// Pass indicates the check passed successfully
	Pass Status = iota
	// Warn indicates a non-critical issue was found
	Warn
	// Fail indicates a critical issue was found
	Fail
)

// String returns a human-readable status string
func (s Status) String() string {
	switch s {
	case Pass:
		return "pass"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	default:
		return "unknown"
	}
}

// CheckResult represents the result of a single check
type CheckResult struct {
	Name    string `json:"name"`    // Name of the check
	Status  Status `json:"status"`  // Result status
	Message string `json:"message"` // Human-readable message
	Details string `json:"details"` // Additional details (optional)
	Fixable bool   `json:"fixable"` // Whether the issue can be auto-fixed
}

// Config holds configuration for running checks
type Config struct {
	Settings   *config.Settings
	CertsDir   string
	ConfigPath string // Path to the config file (for validation)
}

// Check is the interface that all diagnostic checks must implement
type Check interface {
	// Name returns the name of this check
	Name() string

	// Run executes the check and returns the result
	Run(ctx context.Context, cfg *Config) CheckResult

	// CanFix returns whether this check supports auto-fixing
	CanFix() bool

	// Fix attempts to fix the issue found by this check
	// Should only be called if CanFix returns true and Run returned Fail/Warn
	Fix(ctx context.Context, cfg *Config) error
}

// Doctor runs diagnostic checks on the system
type Doctor struct {
	checks []Check
	config *Config
}

// New creates a new Doctor with the given configuration
func New(cfg *Config) *Doctor {
	return &Doctor{
		checks: []Check{},
		config: cfg,
	}
}

// AddCheck adds a check to the doctor
func (d *Doctor) AddCheck(check Check) {
	d.checks = append(d.checks, check)
}

// RunAll runs all registered checks and returns the results
func (d *Doctor) RunAll(ctx context.Context) []CheckResult {
	var results []CheckResult
	for _, check := range d.checks {
		result := check.Run(ctx, d.config)
		results = append(results, result)
	}
	return results
}

// Fix attempts to fix all fixable issues
func (d *Doctor) Fix(ctx context.Context) ([]CheckResult, error) {
	var results []CheckResult

	for _, check := range d.checks {
		result := check.Run(ctx, d.config)

		// If the check failed/warned and is fixable, attempt to fix
		if result.Status != Pass && check.CanFix() {
			if err := check.Fix(ctx, d.config); err != nil {
				result.Details = "Fix failed: " + err.Error()
			} else {
				// Re-run the check to verify the fix worked
				result = check.Run(ctx, d.config)
				if result.Status == Pass {
					result.Details = "Auto-fixed"
				}
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// HasErrors returns true if any check failed
func HasErrors(results []CheckResult) bool {
	for _, r := range results {
		if r.Status == Fail {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any check has warnings
func HasWarnings(results []CheckResult) bool {
	for _, r := range results {
		if r.Status == Warn {
			return true
		}
	}
	return false
}
