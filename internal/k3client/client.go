package k3client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zhoujw/openerp-cli/errs"
)

// Config builds a Client.
type Config struct {
	ServerURL   string // e.g. https://akeparking.ik3cloud.com/K3Cloud/
	AcctID      string
	UserName    string
	AppID       string
	AppSecret   string
	LCID        int
	SessionPath string // file to persist the session id (per profile)
	Verbose     bool
	Log         io.Writer // verbose log sink (stderr)
}

// Client talks to one K3 Cloud instance. Build with New.
type Client struct {
	cfg     Config
	http    *http.Client
	session string // in-memory session id

	// test seams (overridable)
	nowFn func() time.Time
	ridFn func() int64
}

// New builds a Client with sensible defaults.
func New(cfg Config) *Client {
	return &Client{
		cfg:   cfg,
		http:  &http.Client{Timeout: 30 * time.Second},
		nowFn: time.Now,
		ridFn: func() int64 { return rand.Int63() },
	}
}

func (c *Client) logf(format string, a ...any) {
	if c.cfg.Verbose && c.cfg.Log != nil {
		fmt.Fprintln(c.cfg.Log, "[openerp k3] "+fmt.Sprintf(format, a...))
	}
}

func (c *Client) url(endpoint string) string {
	return strings.TrimRight(c.cfg.ServerURL, "/") + "/" + endpoint
}

// loginResp is the LoginBySign result (fields are top-level).
type loginResp struct {
	LoginResultType int             `json:"LoginResultType"`
	KDSVCSessionId  string          `json:"KDSVCSessionId"`
	Message         json.RawMessage `json:"Message"`
	MessageCode     json.RawMessage `json:"MessageCode"`
}

// Login performs LoginBySign and caches/persists the session id. Returns an
// *errs.AuthError on failure (mapped to exit 3).
func (c *Client) Login(ctx context.Context) error {
	ts := strconv.FormatInt(c.nowFn().Unix(), 10)
	sign := sha256SortedSign([]string{c.cfg.AcctID, c.cfg.UserName, c.cfg.AppID, c.cfg.AppSecret, ts})
	params, _ := json.Marshal([]any{c.cfg.AcctID, c.cfg.UserName, c.cfg.AppID, ts, sign, c.cfg.LCID})

	c.session = "" // never send a stale session on the login call
	c.logf("LoginBySign acctId=%s user=%s", c.cfg.AcctID, c.cfg.UserName)
	raw, err := c.post(ctx, EndpointLoginBySign, string(params))
	if err != nil {
		return err
	}
	var lr loginResp
	if e := json.Unmarshal(raw, &lr); e != nil {
		return errs.NewAuth("LoginBySign 响应解析失败: "+snippet(raw),
			"检查 --server-url 是否为有效的 K3Cloud 地址", 0)
	}
	if lr.LoginResultType != 1 {
		msg := strings.TrimSpace(string(lr.Message))
		if msg == "" || msg == "null" {
			msg = snippet(raw)
		}
		return errs.NewAuth("LoginBySign 失败: "+msg,
			"核对 --acct-id / --user / --app-id / --app-secret", msgCode(lr.MessageCode))
	}
	c.session = lr.KDSVCSessionId
	saveSession(c.cfg.SessionPath, Session{SessionID: c.session, AcctID: c.cfg.AcctID, LoginAt: c.nowFn().Unix()})
	c.logf("login ok session=%s", maskSession(c.session))
	return nil
}

// ensureSession hydrates the in-memory session from disk, or logs in.
func (c *Client) ensureSession(ctx context.Context) error {
	if c.session != "" {
		return nil
	}
	if s, ok := loadSession(c.cfg.SessionPath); ok && s.SessionID != "" && s.AcctID == c.cfg.AcctID {
		c.session = s.SessionID
		c.logf("reuse cached session=%s", maskSession(c.session))
		return nil
	}
	return c.Login(ctx)
}

func (c *Client) post(ctx context.Context, endpoint, parametersJSON string) ([]byte, error) {
	body := buildOuterBody(parametersJSON, c.nowFn(), c.ridFn())
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(endpoint), bytes.NewReader(b))
	if err != nil {
		return nil, errs.NewNetwork("构造请求失败: "+err.Error(), "", "conn")
	}
	req.Header.Set("Content-Type", "application/json")
	if c.session != "" {
		req.Header.Set("kdservice-sessionid", c.session)
	}
	t0 := c.nowFn()
	resp, err := c.http.Do(req)
	if err != nil {
		c.logf("POST %s network error: %v", endpoint, err)
		return nil, errs.NewNetwork("请求失败: "+err.Error(),
			"检查网络连通性与 --server-url", networkKind(err))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	c.logf("POST %s -> HTTP %d (%dms, %dB)", endpoint, resp.StatusCode, time.Since(t0).Milliseconds(), len(raw))
	if resp.StatusCode >= 500 {
		return nil, errs.NewNetwork(fmt.Sprintf("网关返回 HTTP %d: %s", resp.StatusCode, snippet(raw)),
			"稍后重试", "5xx")
	}
	return raw, nil
}

// Call sends an authenticated request. If the raw response signals an expired
// session (contains "登录"), it re-logs in once and retries.
func (c *Client) Call(ctx context.Context, endpoint, parametersJSON string) ([]byte, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}
	raw, err := c.post(ctx, endpoint, parametersJSON)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(raw, []byte("登录")) {
		c.logf("session expired, re-login + retry once")
		clearSessionFile(c.cfg.SessionPath)
		c.session = ""
		if err := c.Login(ctx); err != nil {
			return nil, err
		}
		raw, err = c.post(ctx, endpoint, parametersJSON)
		if err != nil {
			return nil, err
		}
		if bytes.Contains(raw, []byte("登录")) {
			return nil, errs.NewAuth("重新登录后请求仍提示需登录",
				"运行 `openerp auth test` 核对凭据", 0)
		}
	}
	return raw, nil
}

// QueryArgs are the inputs to ExecuteBillQuery.
type QueryArgs struct {
	FormID string
	Fields string
	Filter string
	Order  string
	Top    int
	Start  int
	Limit  int
}

// billQueryReq is the inner ExecuteBillQuery payload (field order matches Python).
type billQueryReq struct {
	FormId       string `json:"FormId"`
	FieldKeys    string `json:"FieldKeys"`
	FilterString string `json:"FilterString"`
	OrderString  string `json:"OrderString"`
	TopRowCount  int    `json:"TopRowCount"`
	StartRow     int    `json:"StartRow"`
	Limit        int    `json:"Limit"`
}

// BuildBillQueryParams returns the `parameters` JSON string for an ExecuteBillQuery
// (an array containing one JSON-string element), matching the Python encoding.
func BuildBillQueryParams(q QueryArgs) string {
	inner, _ := json.Marshal(billQueryReq{
		FormId: q.FormID, FieldKeys: q.Fields, FilterString: q.Filter,
		OrderString: q.Order, TopRowCount: q.Top, StartRow: q.Start, Limit: q.Limit,
	})
	params, _ := json.Marshal([]string{string(inner)})
	return string(params)
}

// ExecuteBillQuery runs a list query and returns the raw response (array of rows).
func (c *Client) ExecuteBillQuery(ctx context.Context, q QueryArgs) ([]byte, error) {
	return c.Call(ctx, EndpointExecuteBillQuery, BuildBillQueryParams(q))
}

type viewReq struct {
	Number      string `json:"Number"`
	CreateOrgId int    `json:"CreateOrgId"`
	Id          string `json:"Id"`
}

// BuildViewParams returns the `parameters` JSON string for a View call.
func BuildViewParams(formID, number string) string {
	inner, _ := json.Marshal(viewReq{Number: number, CreateOrgId: 0, Id: ""})
	params, _ := json.Marshal([]any{formID, string(inner)})
	return string(params)
}

// View fetches a single record by number.
func (c *Client) View(ctx context.Context, formID, number string) ([]byte, error) {
	return c.Call(ctx, EndpointView, BuildViewParams(formID, number))
}

// Prepared is what --dry-run prints: the request that would be sent (session masked).
type Prepared struct {
	Endpoint string    `json:"endpoint"`
	URL      string    `json:"url"`
	Session  string    `json:"session"`
	Body     outerBody `json:"body"`
}

// Prepare builds the request artifacts without sending (for --dry-run). It does
// not log in; the shown session is whatever is already cached (possibly empty).
func (c *Client) Prepare(endpoint, parametersJSON string) Prepared {
	if c.session == "" {
		if s, ok := loadSession(c.cfg.SessionPath); ok && s.AcctID == c.cfg.AcctID {
			c.session = s.SessionID
		}
	}
	return Prepared{
		Endpoint: endpoint,
		URL:      c.url(endpoint),
		Session:  maskSession(c.session),
		Body:     buildOuterBody(parametersJSON, c.nowFn(), c.ridFn()),
	}
}

// MaskedSession returns the current in-memory session id, masked.
func (c *Client) MaskedSession() string { return maskSession(c.session) }

func msgCode(raw json.RawMessage) int {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	return 0
}

func networkKind(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "timeout") || strings.Contains(s, "deadline"):
		return "timeout"
	case strings.Contains(s, "no such host") || strings.Contains(s, "dns"):
		return "dns"
	case strings.Contains(s, "tls") || strings.Contains(s, "certificate"):
		return "tls"
	}
	return "conn"
}

func snippet(b []byte) string {
	const max = 300
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
