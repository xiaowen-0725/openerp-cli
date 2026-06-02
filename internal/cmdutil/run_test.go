package cmdutil

import "testing"

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
