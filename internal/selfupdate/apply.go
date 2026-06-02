package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xiaowen-0725/openerp-cli/internal/config"
)

// ErrPermission means the running binary's directory is not writable (e.g. a
// system-wide `npm i -g`). Callers should fall back to telling the user to
// reinstall via npm rather than failing hard.
var ErrPermission = errors.New("无写入权限，无法原子替换当前二进制")

// Result reports what an Apply did.
type Result struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Updated bool   `json:"updated"`
	Path    string `json:"path,omitempty"`
}

// Apply downloads release `version`, verifies its SHA256 against checksums.txt,
// and atomically replaces the currently-running executable. It clears the
// background lock on completion so the next check can act again.
func Apply(ctx context.Context, current, version string) (Result, error) {
	res := Result{From: current, To: version}
	if dir, err := config.Dir(); err == nil {
		defer clearLock(dir)
	}

	exe, err := os.Executable()
	if err != nil {
		return res, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	res.Path = exe

	// Fail fast on an unwritable target dir so callers can route to npm.
	if !dirWritable(filepath.Dir(exe)) {
		return res, ErrPermission
	}

	osName, arch, ext := goTarget()
	if osName == "" || arch == "" {
		return res, fmt.Errorf("不支持的平台: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	asset := fmt.Sprintf("openerp-cli_%s_%s_%s.%s", version, osName, arch, ext)
	base := fmt.Sprintf("https://github.com/%s/releases/download/v%s", Repo, version)

	archive, err := download(ctx, base+"/"+asset)
	if err != nil {
		return res, err
	}
	defer os.Remove(archive)

	sums, err := downloadText(ctx, base+"/checksums.txt")
	if err != nil {
		return res, err
	}
	want, ok := checksumFor(sums, asset)
	if !ok {
		return res, fmt.Errorf("checksums.txt 缺少 %s 的校验和", asset)
	}
	if err := verifySHA256(archive, want); err != nil {
		return res, err
	}

	// Extract the binary next to the target so the final rename is same-filesystem
	// and therefore atomic.
	binName := "openerp"
	if runtime.GOOS == "windows" {
		binName = "openerp.exe"
	}
	staged := exe + ".new"
	if err := extractBinary(archive, ext, binName, staged); err != nil {
		return res, err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		_ = os.Remove(staged)
		return res, err
	}
	if err := replaceExecutable(exe, staged); err != nil {
		_ = os.Remove(staged)
		if os.IsPermission(err) {
			return res, ErrPermission
		}
		return res, err
	}
	res.Updated = true
	return res, nil
}

// download streams url to a temp file and returns its path.
func download(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "openerp-cli-selfupdate")
	resp, err := httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 %s 失败 HTTP %d", filepath.Base(url), resp.StatusCode)
	}
	f, err := os.CreateTemp("", "openerp-update-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func downloadText(ctx context.Context, url string) (string, error) {
	p, err := download(ctx, url)
	if err != nil {
		return "", err
	}
	defer os.Remove(p)
	b, err := os.ReadFile(p)
	return string(b), err
}

// checksumFor finds the hex SHA256 for asset in a goreleaser checksums.txt
// (lines of "<sha256>  <filename>").
func checksumFor(sums, asset string) (string, bool) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

func verifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("校验和不匹配: 期望 %s, 实际 %s", want, got)
	}
	return nil
}

// extractBinary pulls the named member out of a tar.gz or zip archive to dst.
func extractBinary(archive, ext, member, dst string) error {
	if ext == "zip" {
		return extractZip(archive, member, dst)
	}
	return extractTarGz(archive, member, dst)
}

func extractTarGz(archive, member, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == member {
			return writeFileFrom(dst, tr)
		}
	}
	return fmt.Errorf("归档中未找到 %s", member)
}

func extractZip(archive, member, dst string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if filepath.Base(zf.Name) == member {
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeFileFrom(dst, rc)
		}
	}
	return fmt.Errorf("归档中未找到 %s", member)
}

func writeFileFrom(dst string, r io.Reader) error {
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// dirWritable probes write access by creating and removing a temp file.
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".openerp-wtest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
