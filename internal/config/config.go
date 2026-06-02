// Package config stores Kingdee K3 Cloud credential profiles at
// ~/.config/openerp/config.json (0600). It mirrors openydt-cli's profile model
// but with K3 fields. Secrets live plaintext on a 0600 file and are masked in
// `config list`; OS-keychain storage is a documented later step.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zhoujw/openerp-cli/errs"
)

// DefaultLCID is K3's culture id for Simplified Chinese.
const DefaultLCID = 2052

// Profile is one Kingdee K3 Cloud connection.
type Profile struct {
	Name      string `json:"name"`
	ServerURL string `json:"serverUrl"`
	AcctID    string `json:"acctId"`
	UserName  string `json:"userName"`
	AppID     string `json:"appId"`
	AppSecret string `json:"appSecret"`
	LCID      int    `json:"lcid"`
}

// Config is the on-disk root.
type Config struct {
	CurrentProfile string    `json:"currentProfile"`
	Profiles       []Profile `json:"profiles"`
}

// Dir returns the config directory, honoring OPENERP_CONFIG_DIR (tests/CI).
func Dir() (string, error) {
	if d := os.Getenv("OPENERP_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errs.NewConfig("无法定位用户主目录: "+err.Error(), "设置 OPENERP_CONFIG_DIR 指定配置目录", "")
	}
	return filepath.Join(home, ".config", "openerp"), nil
}

// Path returns the config file path.
func Path() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Load reads the config file; a missing file yields an empty Config.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, errs.NewConfig("读取配置失败: "+err.Error(), "检查 "+p+" 权限", "")
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, errs.NewConfig("配置文件不是合法 JSON: "+err.Error(), "修复或删除 "+p, "")
	}
	return &c, nil
}

// Save writes the config (dir 0700, file 0600).
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errs.NewConfig("无法创建配置目录: "+err.Error(), "", "")
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return errs.NewConfig("无法序列化配置: "+err.Error(), "", "")
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), b, 0o600)
}

// Find returns the named profile.
func (c *Config) Find(name string) (*Profile, bool) {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i], true
		}
	}
	return nil, false
}

// Upsert inserts or updates a profile by name.
func (c *Config) Upsert(p Profile) {
	for i := range c.Profiles {
		if c.Profiles[i].Name == p.Name {
			c.Profiles[i] = p
			return
		}
	}
	c.Profiles = append(c.Profiles, p)
}

// applyEnv overlays OPENERP_* overrides onto p.
func applyEnv(p *Profile) {
	if v := os.Getenv("OPENERP_SERVER_URL"); v != "" {
		p.ServerURL = v
	}
	if v := os.Getenv("OPENERP_ACCT_ID"); v != "" {
		p.AcctID = v
	}
	if v := os.Getenv("OPENERP_USER"); v != "" {
		p.UserName = v
	}
	if v := os.Getenv("OPENERP_APP_ID"); v != "" {
		p.AppID = v
	}
	if v := os.Getenv("OPENERP_APP_SECRET"); v != "" {
		p.AppSecret = v
	}
	if v := os.Getenv("OPENERP_LCID"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.LCID = n
		}
	}
}

// Resolve selects a profile (flag > OPENERP_PROFILE > currentProfile), applies
// env overrides, defaults LCID, and validates required fields.
func Resolve(profileFlag string) (Profile, error) {
	cfg, err := Load()
	if err != nil {
		return Profile{}, err
	}
	name := profileFlag
	if name == "" {
		name = os.Getenv("OPENERP_PROFILE")
	}
	if name == "" {
		name = cfg.CurrentProfile
	}

	var p Profile
	if name != "" {
		fp, ok := cfg.Find(name)
		if !ok {
			return Profile{}, errs.NewConfig(fmt.Sprintf("profile %q 不存在", name),
				"用 `openerp config list` 查看，或 `openerp config set` 新建", "profile")
		}
		p = *fp
	}
	applyEnv(&p)
	if p.LCID == 0 {
		p.LCID = DefaultLCID
	}
	if miss := missingField(p); miss != "" {
		return Profile{}, errs.NewConfig("缺少凭据字段: "+miss,
			"运行 `openerp config set --profile <name> --server-url ... --acct-id ... --user ... --app-id ... --app-secret ...`", miss)
	}
	if p.Name == "" {
		p.Name = "default"
	}
	return p, nil
}

func missingField(p Profile) string {
	switch {
	case strings.TrimSpace(p.ServerURL) == "":
		return "serverUrl"
	case strings.TrimSpace(p.AcctID) == "":
		return "acctId"
	case strings.TrimSpace(p.UserName) == "":
		return "userName"
	case strings.TrimSpace(p.AppID) == "":
		return "appId"
	case strings.TrimSpace(p.AppSecret) == "":
		return "appSecret"
	}
	return ""
}

// Mask redacts a secret for display: keep first/last char, star the middle.
func Mask(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 2 {
		return "***"
	}
	return string(r[0]) + "***" + string(r[len(r)-1])
}
