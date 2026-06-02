package selfupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"v1.2.3", "1.2.3", 0},
		{"0.1.0", "0.1.1", -1},
		{"1.0", "1.0.0", 0},        // missing patch defaults to 0
		{"1.0.0-poc", "1.0.0", -1}, // clean release outranks pre-release
		{"1.0.0", "1.0.0-rc1", 1},
		{"2.0.0", "10.0.0", -1}, // numeric, not lexical
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNewer(t *testing.T) {
	if !Newer("0.1.0", "0.2.0") {
		t.Error("0.2.0 should be newer than 0.1.0")
	}
	if Newer("0.2.0", "0.1.0") {
		t.Error("0.1.0 must not be newer than 0.2.0")
	}
	if Newer("1.0.0", "1.0.0") {
		t.Error("equal versions are not newer")
	}
}

func TestIsDevVersion(t *testing.T) {
	dev := []string{"0.1.0-poc", "", "dev", "1.2.3-dirty", "abc", "1.0.0+meta"}
	for _, v := range dev {
		if !IsDevVersion(v) {
			t.Errorf("IsDevVersion(%q) should be true", v)
		}
	}
	rel := []string{"0.1.0", "v1.2.3", "10.20.30", "1.0"}
	for _, v := range rel {
		if IsDevVersion(v) {
			t.Errorf("IsDevVersion(%q) should be false", v)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "abc123  openerp-cli_0.2.0_darwin_arm64.tar.gz\n" +
		"def456  openerp-cli_0.2.0_linux_amd64.tar.gz\n"
	got, ok := checksumFor(sums, "openerp-cli_0.2.0_linux_amd64.tar.gz")
	if !ok || got != "def456" {
		t.Errorf("checksumFor = %q,%v want def456,true", got, ok)
	}
	if _, ok := checksumFor(sums, "missing.tar.gz"); ok {
		t.Error("missing asset should not be found")
	}
}

func TestLatestFrom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.1"}`))
	}))
	defer srv.Close()

	got, err := latestFrom(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.3.1" {
		t.Errorf("latestFrom = %q want 0.3.1 (leading v stripped)", got)
	}
}

func TestLatestFromHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := latestFrom(context.Background(), srv.URL); err == nil {
		t.Error("expected error on HTTP 403")
	}
}
