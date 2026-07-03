package k3client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xiaowen-0725/openerp-cli/errs"
)

func newTestClient(serverURL string) *Client {
	c := New(Config{
		ServerURL: serverURL + "/K3Cloud/",
		AcctID:    "acct1", UserName: "u", AppID: "app1", AppSecret: "sec", LCID: 2052,
		SessionPath: "", // no disk persistence in tests
	})
	c.nowFn = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	c.ridFn = func() int64 { return 42 }
	return c
}

// TestBuildOuterBodyWire locks the china_to_unicode double-encoding: CJK in the
// parameters string becomes \uXXXX, which json.Marshal then backslash-doubles on
// the wire, and round-trips back to the singly-escaped form.
func TestBuildOuterBodyWire(t *testing.T) {
	params := "[\"acct1\",\"追溯系统\",\"app1\"]" // input carries real CJK glyphs
	body := buildOuterBody(params, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), 42)
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"timestamp":"2026-01-02 03:04:05"`) {
		t.Errorf("timestamp missing in %s", got)
	}
	if !strings.Contains(got, `"rid":42`) {
		t.Errorf("rid missing in %s", got)
	}
	// On the wire the escapes are backslash-doubled: \\u8ffd...
	if !strings.Contains(got, "\\\\u8ffd\\\\u6eaf\\\\u7cfb\\\\u7edf") {
		t.Errorf("CJK not escaped on wire: %s", got)
	}
	// Round-trip: the parameters field decodes to the singly-escaped string.
	var rt outerBody
	if err := json.Unmarshal(b, &rt); err != nil {
		t.Fatal(err)
	}
	wantParams := "[\"acct1\",\"\\u8ffd\\u6eaf\\u7cfb\\u7edf\",\"app1\"]"
	if rt.Parameters != wantParams {
		t.Errorf("parameters = %q, want %q", rt.Parameters, wantParams)
	}
}

func TestPrepareDryRun(t *testing.T) {
	c := newTestClient("https://x")
	c.session = "SESSION-ABCDEFGH"
	p := c.Prepare(EndpointView, BuildViewParams("ENG_BOM", "N1"))
	if want := "https://x/K3Cloud/" + EndpointView; p.URL != want {
		t.Errorf("url = %s, want %s", p.URL, want)
	}
	if p.Session != "SESS...EFGH" {
		t.Errorf("masked session = %q, want SESS...EFGH", p.Session)
	}
	if p.Body.Rid != 42 || p.Body.Format != 1 || p.Body.V != "1.0" {
		t.Errorf("unexpected body: %+v", p.Body)
	}
}

// TestExpiryRelogin seeds a stale session so the first business call trips the
// "登录" expiry signal, then asserts exactly one re-login + one retry succeeds.
func TestExpiryRelogin(t *testing.T) {
	var loginCount, bizCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "LoginBySign"):
			loginCount++
			io.WriteString(w, `{"LoginResultType":1,"KDSVCSessionId":"SESS-ABCDEFGH"}`)
		case strings.Contains(r.URL.Path, "ExecuteBillQuery"):
			bizCount++
			if bizCount == 1 {
				io.WriteString(w, `{"Message":"会话已超时,请重新登录"}`)
			} else {
				io.WriteString(w, `[["FNUM-1"]]`)
			}
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.session = "STALE-SESSION" // seed: first business call carries it → expiry
	raw, err := c.ExecuteBillQuery(context.Background(), QueryArgs{FormID: "ENG_BOM", Fields: "FNumber", Top: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(raw) != `[["FNUM-1"]]` {
		t.Fatalf("raw = %s", raw)
	}
	if loginCount != 1 {
		t.Errorf("loginCount = %d, want 1", loginCount)
	}
	if bizCount != 2 {
		t.Errorf("bizCount = %d, want 2", bizCount)
	}
}

// TestLoginFail maps LoginResultType!=1 to a typed *errs.AuthError (exit 3).
func TestLoginFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"LoginResultType":0,"Message":"用户名或密码错误"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.Login(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *errs.AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("want *errs.AuthError, got %T: %v", err, err)
	}
}

// TestAttachmentDownLoadChunkBusinessError maps IsSuccess=false to a typed
// *errs.APIError (exit 1), surfacing the K3 error message.
func TestAttachmentDownLoadChunkBusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"Result":{"ResponseStatus":{"IsSuccess":false,"ErrorCode":500,"Errors":[{"Message":"文件不存在"}]}}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.session = "SESS-X"
	_, err := c.AttachmentDownLoadChunk(context.Background(), "nope", 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *errs.APIError
	if !errors.As(err, &ae) {
		t.Fatalf("want *errs.APIError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "文件不存在") {
		t.Errorf("error should carry K3 message, got: %v", err)
	}
}

// TestAttachmentDownLoadChunkMultiBlock serves a 2-chunk download (verified chunk
// size is 1MB) and asserts the caller can loop StartIndex → IsLast to reassemble
// the decoded bytes. Models the RunAttachmentDownLoad loop in cmdutil.
func TestAttachmentDownLoadChunkMultiBlock(t *testing.T) {
	first := strings.Repeat("A", 1048576) // 1MB, fits one base64 chunk
	second := "B"                         // tiny tail
	full := first + second
	b64First := base64.StdEncoding.EncodeToString([]byte(first))
	b64Second := base64.StdEncoding.EncodeToString([]byte(second))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		raw, _ := io.ReadAll(req.Body)
		var body struct {
			Parameters string `json:"parameters"`
		}
		_ = json.Unmarshal(raw, &body)
		var params []string
		_ = json.Unmarshal([]byte(body.Parameters), &params)
		var inner struct {
			StartIndex int64 `json:"StartIndex"`
		}
		_ = json.Unmarshal([]byte(params[0]), &inner)

		if inner.StartIndex == 0 {
			io.WriteString(w, fmt.Sprintf(`{"Result":{"ResponseStatus":{"IsSuccess":true},"StartIndex":1048576,"IsLast":false,"FileSize":%d,"FileName":"x.zip","FilePart":"%s"}}`, len(full), b64First))
		} else {
			io.WriteString(w, fmt.Sprintf(`{"Result":{"ResponseStatus":{"IsSuccess":true},"StartIndex":%d,"IsLast":true,"FileSize":%d,"FileName":"x.zip","FilePart":"%s"}}`, len(full), len(full), b64Second))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.session = "SESS-X"

	var assembled []byte
	start := int64(0)
	chunks := 0
	for {
		r, err := c.AttachmentDownLoadChunk(context.Background(), "fid", start)
		if err != nil {
			t.Fatalf("chunk %d: %v", chunks, err)
		}
		part, derr := base64.StdEncoding.DecodeString(r.FilePart)
		if derr != nil {
			t.Fatalf("decode chunk %d: %v", chunks, derr)
		}
		assembled = append(assembled, part...)
		chunks++
		if r.IsLast {
			break
		}
		start = r.StartIndex
	}
	if chunks != 2 {
		t.Errorf("chunks = %d, want 2", chunks)
	}
	if len(assembled) != len(full) {
		t.Errorf("assembled len = %d, want %d", len(assembled), len(full))
	}
	if string(assembled) != full {
		t.Errorf("assembled content mismatch")
	}
}
