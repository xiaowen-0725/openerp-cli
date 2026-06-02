package cmdutil

import (
	"path/filepath"
	"strings"

	"github.com/zhoujw/openerp-cli/internal/config"
	"github.com/zhoujw/openerp-cli/internal/k3client"
)

// Factory carries global flags and lazily builds shared dependencies. Every
// command receives *Factory and goes through it for config/client/IO.
type Factory struct {
	IOStreams *IOStreams

	// Global flags (bound on the root command's persistent flag set).
	Profile  string
	Env      string // reserved (instance URL is per-profile, not per-env)
	Format   string
	Jq       string
	DryRun   bool
	Verbose  bool
	ReadOnly bool
}

// NewFactory builds the default production factory.
func NewFactory() *Factory {
	return &Factory{IOStreams: SystemIOStreams(), Format: "json"}
}

// Config resolves the active profile (flag > env > current), applying env
// overrides and validating required fields.
func (f *Factory) Config() (config.Profile, error) {
	return config.Resolve(f.Profile)
}

// Client builds a K3 client for the resolved profile.
func (f *Factory) Client() (*k3client.Client, error) {
	p, err := f.Config()
	if err != nil {
		return nil, err
	}
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	sessionPath := filepath.Join(dir, "session-"+sanitize(p.Name)+".json")
	return k3client.New(k3client.Config{
		ServerURL:   p.ServerURL,
		AcctID:      p.AcctID,
		UserName:    p.UserName,
		AppID:       p.AppID,
		AppSecret:   p.AppSecret,
		LCID:        p.LCID,
		SessionPath: sessionPath,
		Verbose:     f.Verbose,
		Log:         f.IOStreams.Err,
	}), nil
}

// sanitize makes a profile name safe for a filename.
func sanitize(name string) string {
	if name == "" {
		return "default"
	}
	repl := func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}
	return strings.Map(repl, name)
}
