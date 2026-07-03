package cmdutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaowen-0725/openerp-cli/errs"
)

func TestEnsureFields(t *testing.T) {
	got := ensureFields("FBillNo,FDate", "FCustId.FName", "FBillAllAmount_LC", "FDate")
	want := "FBillNo,FDate,FCustId.FName,FBillAllAmount_LC"
	if got != want {
		t.Errorf("ensureFields = %q, want %q", got, want)
	}
}

func TestAggregateSum(t *testing.T) {
	fields := []string{"FBillNo", "FCust", "FAmt"}
	rows := []interface{}{
		[]interface{}{"A", "X", float64(100)},
		[]interface{}{"B", "Y", float64(50)},
		[]interface{}{"C", "X", "30"}, // string-number coerced
	}
	res, _, err := aggregate(fields, rows, "FAmt", "")
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]interface{})
	if m["count"].(int) != 3 {
		t.Errorf("count = %v, want 3", m["count"])
	}
	if m["sum"].(float64) != 180 {
		t.Errorf("sum = %v, want 180", m["sum"])
	}
}

func TestAggregateGroupSortedDesc(t *testing.T) {
	fields := []string{"FBillNo", "FCust", "FAmt"}
	rows := []interface{}{
		[]interface{}{"A", "X", float64(100)},
		[]interface{}{"B", "Y", float64(50)},
		[]interface{}{"C", "X", float64(30)},
	}
	res, meta, err := aggregate(fields, rows, "FAmt", "FCust")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Count != 2 {
		t.Errorf("meta.Count = %d, want 2", meta.Count)
	}
	groups := res.([]interface{})
	g0 := groups[0].(map[string]interface{})
	if g0["group"] != "X" || g0["sum"].(float64) != 130 || g0["count"].(int) != 2 {
		t.Errorf("top group = %v, want X/130/2", g0)
	}
}

func TestAggregateMissingField(t *testing.T) {
	if _, _, err := aggregate([]string{"FBillNo"}, nil, "FNope", ""); err == nil {
		t.Error("expected error for --sum field not in columns")
	}
}

// TestResolveOutPath covers the four branches: default name, dir join, explicit
// file, and the exists-without-overwrite guard. Uses t.TempDir for isolation.
func TestResolveOutPath(t *testing.T) {
	dir := t.TempDir()

	// 1) empty outFlag → ./<serverName>; resolve to abs for the existence check.
	got, err := resolveOutPath("", "a.zip", false)
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if !strings.HasSuffix(got, "/a.zip") {
		t.Errorf("default path = %s, want suffix /a.zip", got)
	}

	// 2) outFlag is a dir → join.
	got, err = resolveOutPath(dir, "b.zip", false)
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	if want := filepath.Join(dir, "b.zip"); got != want {
		t.Errorf("dir path = %s, want %s", got, want)
	}

	// 3) outFlag is an explicit file path.
	explicit := filepath.Join(dir, "custom.zip")
	got, err = resolveOutPath(explicit, "ignored.zip", false)
	if err != nil {
		t.Fatalf("explicit: %v", err)
	}
	if got != explicit {
		t.Errorf("explicit path = %s, want %s", got, explicit)
	}

	// 4) target exists, overwrite=false → *ValidationError.
	existing := filepath.Join(dir, "exists.zip")
	if f, e := os.Create(existing); e == nil {
		f.Close()
	} else {
		t.Fatalf("setup: %v", e)
	}
	_, err = resolveOutPath(existing, "x.zip", false)
	if err == nil {
		t.Fatal("expected error for existing file without overwrite")
	}
	if _, ok := errs.ProblemOf(err); !ok {
		t.Fatalf("want typed errs error, got %T: %v", err, err)
	}

	// 5) same target with overwrite=true → ok.
	got, err = resolveOutPath(existing, "x.zip", true)
	if err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if got != existing {
		t.Errorf("overwrite path = %s, want %s", got, existing)
	}
}
