package config

import (
	"path/filepath"
	"testing"
)

func TestMask(t *testing.T) {
	cases := map[string]string{
		"":       "",
		"ab":     "***",
		"abcdef": "a***f",
	}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}

func writeProfile(t *testing.T, dir string) {
	t.Helper()
	cfg := &Config{
		CurrentProfile: "prod",
		Profiles: []Profile{{
			Name: "prod", ServerURL: "https://a/K3Cloud/", AcctID: "acct",
			UserName: "u", AppID: "app", AppSecret: "sec", LCID: 2052,
		}},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if p, _ := Path(); p != filepath.Join(dir, "config.json") {
		t.Fatalf("path = %s", p)
	}
}

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENERP_CONFIG_DIR", dir)
	t.Setenv("OPENERP_PROFILE", "")
	t.Setenv("OPENERP_SERVER_URL", "")
	t.Setenv("OPENERP_APP_SECRET", "")
	writeProfile(t, dir)

	p, err := Resolve("")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Name != "prod" || p.ServerURL != "https://a/K3Cloud/" || p.LCID != 2052 {
		t.Fatalf("resolved = %+v", p)
	}

	// env override beats stored profile
	t.Setenv("OPENERP_APP_SECRET", "override-secret")
	p2, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if p2.AppSecret != "override-secret" {
		t.Errorf("appSecret = %q, want override-secret", p2.AppSecret)
	}
}

func TestResolveMissingProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENERP_CONFIG_DIR", dir)
	t.Setenv("OPENERP_PROFILE", "")
	t.Setenv("OPENERP_SERVER_URL", "")
	t.Setenv("OPENERP_ACCT_ID", "")
	t.Setenv("OPENERP_USER", "")
	t.Setenv("OPENERP_APP_ID", "")
	t.Setenv("OPENERP_APP_SECRET", "")
	if _, err := Resolve(""); err == nil {
		t.Fatal("expected a config error when no profile/credentials exist")
	}
}
