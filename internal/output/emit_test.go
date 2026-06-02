package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/xiaowen-0725/openerp-cli/errs"
)

func TestExitCodeForCategory(t *testing.T) {
	cases := map[errs.Category]int{
		errs.CategoryValidation:   ExitValidation,
		errs.CategoryAuth:         ExitAuth,
		errs.CategoryConfig:       ExitAuth,
		errs.CategoryNetwork:      ExitNetwork,
		errs.CategoryAPI:          ExitAPI,
		errs.CategoryInternal:     ExitInternal,
		errs.CategoryConfirmation: ExitConfirmationRequired,
	}
	for cat, want := range cases {
		if got := ExitCodeForCategory(cat); got != want {
			t.Errorf("ExitCodeForCategory(%s) = %d, want %d", cat, got, want)
		}
	}
}

func TestExitCodeOf(t *testing.T) {
	if got := ExitCodeOf(errs.NewValidation("x", "y", "z")); got != ExitValidation {
		t.Errorf("typed validation -> %d, want %d", got, ExitValidation)
	}
	if got := ExitCodeOf(errors.New("plain")); got != ExitInternal {
		t.Errorf("untyped -> %d, want %d", got, ExitInternal)
	}
	if got := ExitCodeOf(nil); got != ExitOK {
		t.Errorf("nil -> %d, want %d", got, ExitOK)
	}
}

func TestEmitError(t *testing.T) {
	var buf bytes.Buffer
	code := EmitError(&buf, errs.NewValidation("缺少 --form-id", "如 --form-id ENG_BOM", "form-id"))
	if code != ExitValidation {
		t.Errorf("code = %d, want %d", code, ExitValidation)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("error envelope not JSON: %v", err)
	}
	if m["ok"] != false {
		t.Errorf("ok = %v, want false", m["ok"])
	}
	if m["type"] != "validation" {
		t.Errorf("type = %v, want validation", m["type"])
	}
	if m["param"] != "form-id" {
		t.Errorf("param = %v, want form-id", m["param"])
	}
	if m["hint"] == "" || m["hint"] == nil {
		t.Errorf("hint missing")
	}
}

func TestEmitErrorSilent(t *testing.T) {
	var buf bytes.Buffer
	code := EmitError(&buf, SilentExit{Code: 7})
	if code != 7 {
		t.Errorf("code = %d, want 7", code)
	}
	if buf.Len() != 0 {
		t.Errorf("SilentExit should print nothing, got %q", buf.String())
	}
}

func TestApplyJQPath(t *testing.T) {
	var v interface{}
	_ = json.Unmarshal([]byte(`{"ok":true,"data":[{"FName":"A"},{"FName":"B"}]}`), &v)

	got, err := applyJQPath(v, ".data[1].FName")
	if err != nil {
		t.Fatal(err)
	}
	if got != "B" {
		t.Errorf("got %v, want B", got)
	}
	arr, err := applyJQPath(v, ".data")
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := arr.([]interface{}); !ok || len(a) != 2 {
		t.Errorf(".data not a 2-element array: %v", arr)
	}
	if _, err := applyJQPath(v, "data"); err == nil {
		t.Error("expected error for path without leading dot")
	}
	if _, err := applyJQPath(v, ".data[9]"); err == nil {
		t.Error("expected out-of-range error")
	}
}

func TestEmitDataJQ(t *testing.T) {
	var buf bytes.Buffer
	data := []interface{}{map[string]interface{}{"FName": "A"}}
	if err := EmitData(&buf, "json", ".data[0].FName", data, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `"A"` {
		t.Errorf("jq output = %q, want \"A\"", got)
	}
}
