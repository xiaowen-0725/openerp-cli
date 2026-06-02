// Package k3client is the Kingdee K3 Cloud (金蝶云·星空) WebAPI client. Unlike a
// stateless per-request signing scheme, K3 logs in ONCE via LoginBySign, gets a
// KDSVCSessionId, and reuses it via the `kdservice-sessionid` header until it
// expires. This package ports the debugged logic from ai_jieti's Python client
// (LoginBySign + china_to_unicode quirk + ExecuteBillQuery/View + auto re-login).
package k3client

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// WebAPI endpoint paths (appended to ServerURL, which ends in /K3Cloud/).
const (
	EndpointLoginBySign      = "Kingdee.BOS.WebApi.ServicesStub.AuthService.LoginBySign.common.kdsvc"
	EndpointExecuteBillQuery = "Kingdee.BOS.WebApi.ServicesStub.DynamicFormService.ExecuteBillQuery.common.kdsvc"
	EndpointView             = "Kingdee.BOS.WebApi.ServicesStub.DynamicFormService.View.common.kdsvc"
)

// chinaToUnicode replaces every CJK rune in 0x4E00..0x29FA5 with a lower-case
// \uXXXX escape, leaving all other runes untouched. It is applied to the
// `parameters` string only (a K3 quirk), reproducing the Python helper. This
// substitutes for Python's json.dumps(ensure_ascii=True) over that field.
func chinaToUnicode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x29FA5 {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sha256SortedSign sorts the parts lexicographically, then SHA256-updates each in
// order, returning the lower-case hex digest (ports Python's sha256_sign).
func sha256SortedSign(parts []string) string {
	sorted := make([]string, len(parts))
	copy(sorted, parts)
	sort.Strings(sorted)
	h := sha256.New()
	for _, p := range sorted {
		_, _ = h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// outerBody is K3's request envelope. `parameters` is the chinaToUnicode'd inner
// payload string; json.Marshal then escapes its backslashes on the wire, exactly
// as Python's requests(json=...) does.
type outerBody struct {
	Format     int    `json:"format"`
	UserAgent  string `json:"useragent"`
	Rid        int64  `json:"rid"`
	Parameters string `json:"parameters"`
	Timestamp  string `json:"timestamp"`
	V          string `json:"v"`
}

func buildOuterBody(parametersJSON string, now time.Time, rid int64) outerBody {
	return outerBody{
		Format:     1,
		UserAgent:  "ApiClient",
		Rid:        rid,
		Parameters: chinaToUnicode(parametersJSON),
		Timestamp:  now.Format("2006-01-02 15:04:05"),
		V:          "1.0",
	}
}
