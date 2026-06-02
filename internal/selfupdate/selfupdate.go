// Package selfupdate keeps the openerp binary current against GitHub Releases.
//
// Two entry points share one core:
//   - MaybeBackgroundUpdate: called from the root PreRun on every run. It is
//     throttled (once/day), fully silent, and on finding a newer release spawns
//     a DETACHED child (`openerp __selfupdate-apply <ver>`) that downloads and
//     swaps the binary in place. The foreground command finishes untouched with
//     the old binary; the next invocation picks up the new one. stdout (the JSON
//     contract for agents) is never touched.
//   - Apply: the actual download → checksum-verify → atomic-replace, used by both
//     the detached child and the explicit `openerp update` command.
//
// Releases are the source of truth (matches npm/install.js): assets live at
// https://github.com/<repo>/releases/download/v<ver>/<name>, verified against
// checksums.txt (SHA256) before the running binary is replaced.
package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/xiaowen-0725/openerp-cli/internal/config"
)

// Repo is the GitHub owner/name that publishes releases.
const Repo = "xiaowen-0725/openerp-cli"

// checkInterval throttles the background version check (per machine, per day).
const checkInterval = 24 * time.Hour

// lockTTL suppresses re-spawning the updater while one may still be running.
const lockTTL = 10 * time.Minute

// cacheState persists the last check so we probe GitHub at most once/day.
type cacheState struct {
	LastCheck      time.Time `json:"last_check"`
	LatestVersion  string    `json:"latest_version"`
	CurrentVersion string    `json:"current_version"`
}

// MaybeBackgroundUpdate runs the throttled, silent check and, if a newer release
// exists, spawns a detached updater. It never returns an error and never writes
// to stdout/stderr — auto-update must not perturb a normal command.
func MaybeBackgroundUpdate(current string) {
	defer func() { _ = recover() }() // auto-update is best-effort; never crash a command

	if !autoUpdateEnabled(current) {
		return
	}
	dir, err := config.Dir()
	if err != nil {
		return
	}
	st, _ := loadCache(dir)
	if time.Since(st.LastCheck) < checkInterval {
		// Acted recently. If the cached latest is already known-newer and no
		// updater is in flight, (re)spawn; otherwise stay quiet.
		if Newer(current, st.LatestVersion) && !lockFresh(dir) {
			spawnUpdater(dir, st.LatestVersion)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	latest, err := LatestVersion(ctx)
	if err != nil {
		return // offline / rate-limited: try again after the interval
	}
	st = cacheState{LastCheck: time.Now(), LatestVersion: latest, CurrentVersion: current}
	_ = saveCache(dir, st)

	if Newer(current, latest) && !lockFresh(dir) {
		spawnUpdater(dir, latest)
	}
}

// autoUpdateEnabled gates silent updates off for dev builds and explicit opt-out.
func autoUpdateEnabled(current string) bool {
	if isOptOut() {
		return false
	}
	if IsDevVersion(current) {
		return false
	}
	return true
}

// isOptOut honors OPENERP_NO_UPDATE / OPENERP_NO_UPDATE_CHECK (CI, locked-down hosts).
func isOptOut() bool {
	for _, k := range []string{"OPENERP_NO_UPDATE", "OPENERP_NO_UPDATE_CHECK"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			return true
		}
	}
	return false
}

// IsDevVersion reports whether v is a local/dev build that must not self-update.
// goreleaser stamps clean X.Y.Z tags; anything with a pre-release/metadata suffix
// (e.g. 0.1.0-poc) or an unparseable core is treated as dev.
func IsDevVersion(v string) bool {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if v == "" {
		return true
	}
	if strings.ContainsAny(v, "-+") {
		return true
	}
	_, ok := parseCore(v)
	return !ok
}

// spawnUpdater starts a detached `openerp __selfupdate-apply <ver>`, logging to
// the config dir, and does not wait. A lock file guards against piling up.
func spawnUpdater(dir, version string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if err := writeLock(dir); err != nil {
		return
	}
	logf, _ := os.OpenFile(filepath.Join(dir, "update.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)

	cmd := exec.Command(exe, "__selfupdate-apply", version)
	cmd.Stdin = nil
	if logf != nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
	}
	cmd.SysProcAttr = detachSysProcAttr() // own session/group so we outlive the parent
	if err := cmd.Start(); err != nil {
		if logf != nil {
			_ = logf.Close()
		}
		return
	}
	_ = cmd.Process.Release()
}

// ---- version comparison -------------------------------------------------

// Newer reports whether latest is a strictly newer release than current.
// A dev current (unparseable / pre-release) is never auto-upgraded.
func Newer(current, latest string) bool {
	return Compare(current, latest) < 0
}

// Compare orders two versions: -1 if a<b, 0 if equal, 1 if a>b. It compares the
// numeric dotted core; a pre-release suffix sorts BELOW the same clean core
// (1.0.0-rc < 1.0.0). Unparseable cores sort lowest.
func Compare(a, b string) int {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")
	ac, aPre := splitPre(a)
	bc, bPre := splitPre(b)
	an, aok := parseCore(ac)
	bn, bok := parseCore(bc)
	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return -1
	case !bok:
		return 1
	}
	for i := 0; i < 3; i++ {
		if an[i] != bn[i] {
			if an[i] < bn[i] {
				return -1
			}
			return 1
		}
	}
	// Equal cores: a clean release outranks a pre-release of the same core.
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "":
		return 1
	case bPre == "":
		return -1
	default:
		return strings.Compare(aPre, bPre)
	}
}

func splitPre(v string) (core, pre string) {
	v = strings.SplitN(v, "+", 2)[0] // drop build metadata
	if i := strings.IndexByte(v, '-'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

// parseCore parses "X.Y.Z" (missing trailing parts default to 0) into [3]int.
func parseCore(core string) ([3]int, bool) {
	var out [3]int
	if core == "" {
		return out, false
	}
	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// ---- GitHub latest release ---------------------------------------------

// LatestVersion returns the latest published release version (no leading "v").
func LatestVersion(ctx context.Context) (string, error) {
	return latestFrom(ctx, fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo))
}

// latestFrom is the testable core of LatestVersion (endpoint injected).
func latestFrom(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "openerp-cli-selfupdate")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub releases API HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	v := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if v == "" {
		return "", errors.New("release has empty tag_name")
	}
	return v, nil
}

func httpClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// ---- cache & lock -------------------------------------------------------

func cachePath(dir string) string { return filepath.Join(dir, "update-check.json") }
func lockPath(dir string) string  { return filepath.Join(dir, "update.lock") }

func loadCache(dir string) (cacheState, error) {
	var st cacheState
	b, err := os.ReadFile(cachePath(dir))
	if err != nil {
		return st, err
	}
	_ = json.Unmarshal(b, &st)
	return st, nil
}

func saveCache(dir string, st cacheState) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(cachePath(dir), append(b, '\n'), 0o600)
}

func lockFresh(dir string) bool {
	fi, err := os.Stat(lockPath(dir))
	if err != nil {
		return false
	}
	return time.Since(fi.ModTime()) < lockTTL
}

func writeLock(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(lockPath(dir), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o600)
}

func clearLock(dir string) { _ = os.Remove(lockPath(dir)) }

// platform target strings matching goreleaser asset names.
func goTarget() (osName, arch, ext string) {
	osName = map[string]string{"darwin": "darwin", "linux": "linux", "windows": "windows"}[runtime.GOOS]
	arch = runtime.GOARCH // amd64 / arm64 already match
	ext = "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return
}
