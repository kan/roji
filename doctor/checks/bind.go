package checks

import (
	"context"
	"strings"

	"github.com/kan/roji/config"
	"github.com/kan/roji/doctor"
	"github.com/kan/roji/i18n"
)

// Bind reports which addresses roji listens on. See
// config.Settings.ListensBeyondLoopback for why that is worth reporting.
type Bind struct{}

func (c *Bind) Name() string {
	return i18n.T("doctor.check.bind")
}

func (c *Bind) Run(ctx context.Context, cfg *doctor.Config) doctor.CheckResult {
	if cfg.Settings == nil {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Fail,
			Message: i18n.T("doctor.check.bind.not_available"),
			Fixable: false,
		}
	}

	addrs := describeBind(cfg.Settings.BindAddrs())

	if !cfg.Settings.ListensBeyondLoopback() {
		return doctor.CheckResult{
			Name:    c.Name(),
			Status:  doctor.Pass,
			Message: i18n.Tf("doctor.check.bind.loopback", addrs),
			Fixable: false,
		}
	}

	// Pass, not Warn: the default is loopback, so a wider bind is always
	// something the operator asked for, and Warn would make `roji doctor` exit
	// non-zero forever with nothing to fix.
	return doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.Pass,
		Message: i18n.Tf("doctor.check.bind.exposed", addrs),
		Details: i18n.T("doctor.check.bind.exposed_hint"),
		Fixable: false,
	}
}

func (c *Bind) CanFix() bool {
	return false
}

func (c *Bind) Fix(ctx context.Context, cfg *doctor.Config) error {
	return nil
}

// describeBind renders the bind list for a message, naming the wildcard rather
// than showing it as the empty string it is stored as.
func describeBind(addrs []string) string {
	shown := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if config.IsWildcardAddr(a) {
			a = i18n.T("doctor.check.bind.all_interfaces")
		}
		shown = append(shown, a)
	}
	return strings.Join(shown, ", ")
}
