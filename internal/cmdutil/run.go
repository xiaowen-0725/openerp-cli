package cmdutil

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/k3client"
	"github.com/xiaowen-0725/openerp-cli/internal/output"
)

// QueryOpts controls list/query execution: pagination and optional client-side
// aggregation (--sum / --group-by). Aggregation forces a full fetch.
type QueryOpts struct {
	PageAll   bool
	PageLimit int
	Sum       string // sum this field across the result set
	GroupBy   string // group rows by this field (combine with Sum)
}

// RunBillQuery executes an ExecuteBillQuery, honoring --dry-run, --page-all and
// --sum/--group-by aggregation, and emits the result. Shared by `query`, the
// catalog domain `list` commands, and `bom list`.
func (f *Factory) RunBillQuery(ctx context.Context, q k3client.QueryArgs, o QueryOpts) error {
	c, err := f.Client()
	if err != nil {
		return err
	}
	agg := o.Sum != "" || o.GroupBy != ""
	if agg {
		q.Fields = ensureFields(q.Fields, o.GroupBy, o.Sum) // ensure agg columns are selected
	}
	if f.DryRun {
		p := c.Prepare(k3client.EndpointExecuteBillQuery, k3client.BuildBillQueryParams(q))
		return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, p, nil)
	}
	if o.PageAll || agg { // aggregation needs the full result set
		all, err := f.fetchAllRows(ctx, c, q, o.PageLimit)
		if err != nil {
			return err
		}
		if agg {
			res, meta, err := aggregate(splitFields(q.Fields), all, o.Sum, o.GroupBy)
			if err != nil {
				return err
			}
			return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, res, meta)
		}
		return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, all, &output.Meta{Count: len(all)})
	}
	raw, err := c.ExecuteBillQuery(ctx, q)
	if err != nil {
		return err
	}
	data := decode(raw)
	if apiErr := apiErrorIfAny(data); apiErr != nil {
		return apiErr
	}
	return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, data, metaOf(data))
}

// fetchAllRows pages through ExecuteBillQuery and returns every row.
func (f *Factory) fetchAllRows(ctx context.Context, c *k3client.Client, q k3client.QueryArgs, pageLimit int) ([]interface{}, error) {
	pageSize := q.Limit
	if pageSize <= 0 {
		pageSize = 2000
	}
	start := q.Start
	var all []interface{}
	for page := 0; pageLimit <= 0 || page < pageLimit; page++ {
		pq := q
		pq.Start = start
		pq.Limit = pageSize
		pq.Top = 0
		raw, err := c.ExecuteBillQuery(ctx, pq)
		if err != nil {
			return nil, err
		}
		data := decode(raw)
		if apiErr := apiErrorIfAny(data); apiErr != nil {
			return nil, apiErr
		}
		rows, ok := data.([]interface{})
		if !ok {
			break
		}
		all = append(all, rows...)
		if len(rows) < pageSize {
			break
		}
		start += len(rows)
	}
	return all, nil
}

func splitFields(s string) []string {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// ensureFields appends each non-empty extra field not already present in the
// comma-separated field list (so --sum/--group-by columns are queried).
func ensureFields(csv string, extra ...string) string {
	have := map[string]bool{}
	for _, p := range splitFields(csv) {
		have[p] = true
	}
	for _, e := range extra {
		if e != "" && !have[e] {
			csv += "," + e
			have[e] = true
		}
	}
	return csv
}

// aggregate computes --sum / --group-by over decoded rows. fields is the ordered
// column-name list (from FieldKeys). Returns a grand-total object (no group) or a
// sum-desc–sorted []group, plus meta.
func aggregate(fields []string, rows []interface{}, sumF, groupF string) (interface{}, *output.Meta, error) {
	idx := func(name string) int {
		for i, fn := range fields {
			if fn == name {
				return i
			}
		}
		return -1
	}
	si, gi := -1, -1
	if sumF != "" {
		if si = idx(sumF); si < 0 {
			return nil, nil, errs.NewValidation("--sum 字段不在结果列中: "+sumF, "确认字段名(可用 schema 查)，或写进 --fields", "sum")
		}
	}
	if groupF != "" {
		if gi = idx(groupF); gi < 0 {
			return nil, nil, errs.NewValidation("--group-by 字段不在结果列中: "+groupF, "确认字段名(可用 schema 查)，或写进 --fields", "group-by")
		}
	}
	num := func(v interface{}) float64 {
		switch x := v.(type) {
		case float64:
			return x
		case string:
			fv, _ := strconv.ParseFloat(x, 64)
			return fv
		}
		return 0
	}
	cell := func(r []interface{}, i int) interface{} {
		if i >= 0 && i < len(r) {
			return r[i]
		}
		return nil
	}
	asRow := func(v interface{}) []interface{} { r, _ := v.([]interface{}); return r }

	if gi < 0 {
		var sum float64
		for _, rr := range rows {
			if si >= 0 {
				sum += num(cell(asRow(rr), si))
			}
		}
		res := map[string]interface{}{"count": len(rows)}
		if si >= 0 {
			res["sumField"] = sumF
			res["sum"] = sum
		}
		return res, nil, nil
	}
	type ag struct {
		count int
		sum   float64
	}
	m := map[string]*ag{}
	var order []string
	for _, rr := range rows {
		r := asRow(rr)
		key := fmt.Sprint(cell(r, gi))
		a := m[key]
		if a == nil {
			a = &ag{}
			m[key] = a
			order = append(order, key)
		}
		a.count++
		if si >= 0 {
			a.sum += num(cell(r, si))
		}
	}
	groups := make([]interface{}, 0, len(order))
	for _, k := range order {
		a := m[k]
		g := map[string]interface{}{"group": k, "count": a.count}
		if si >= 0 {
			g["sum"] = a.sum
		}
		groups = append(groups, g)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		a := groups[i].(map[string]interface{})
		b := groups[j].(map[string]interface{})
		if si >= 0 {
			return a["sum"].(float64) > b["sum"].(float64)
		}
		return a["count"].(int) > b["count"].(int)
	})
	return groups, &output.Meta{Count: len(groups)}, nil
}

// RunView executes a View call, honoring --dry-run, and emits the result.
func (f *Factory) RunView(ctx context.Context, formID, number string) error {
	c, err := f.Client()
	if err != nil {
		return err
	}
	if f.DryRun {
		p := c.Prepare(k3client.EndpointView, k3client.BuildViewParams(formID, number))
		return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, p, nil)
	}
	raw, err := c.View(ctx, formID, number)
	if err != nil {
		return err
	}
	data := decode(raw)
	if apiErr := apiErrorIfAny(data); apiErr != nil {
		return apiErr
	}
	return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, data, nil)
}

// RunBusinessInfo executes a QueryBusinessInfo (metadata/field discovery),
// honoring --dry-run. When fieldsOnly is true it emits the compact parsed field
// list; otherwise the full raw metadata. Used by `schema`.
func (f *Factory) RunBusinessInfo(ctx context.Context, formID string, fieldsOnly bool) error {
	c, err := f.Client()
	if err != nil {
		return err
	}
	if f.DryRun {
		p := c.Prepare(k3client.EndpointQueryBusinessInfo, k3client.BuildBusinessInfoParams(formID))
		return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, p, nil)
	}
	raw, err := c.QueryBusinessInfo(ctx, formID)
	if err != nil {
		return err
	}
	data := decode(raw)
	if apiErr := apiErrorIfAny(data); apiErr != nil {
		return apiErr
	}
	if fieldsOnly {
		if bi, perr := k3client.ParseBusinessInfo(raw); perr == nil {
			return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, bi, nil)
		}
	}
	return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, data, nil)
}

func decode(raw []byte) interface{} {
	var v interface{}
	if json.Unmarshal(raw, &v) == nil {
		return v
	}
	return string(raw)
}

func metaOf(data interface{}) *output.Meta {
	if rows, ok := data.([]interface{}); ok {
		return &output.Meta{Count: len(rows)}
	}
	return nil
}

// apiErrorIfAny detects a K3 "ResponseStatus:{IsSuccess:false}" envelope and
// surfaces it as a typed APIError (exit 1). It handles both the View shape
// ({Result:{ResponseStatus:...}}) and the ExecuteBillQuery error shape, where
// the error object is wrapped inside the row array ([[{Result:{...}}]]).
// Successful ExecuteBillQuery rows hold scalar cells, so there is no
// false-positive. Plain-string K3 errors are not caught (a documented POC limit).
func apiErrorIfAny(data interface{}) error {
	rs := findResponseStatus(data)
	if rs == nil {
		return nil
	}
	if success, _ := rs["IsSuccess"].(bool); !success {
		return errs.NewAPI("K3 返回业务错误 (IsSuccess=false)",
			"检查 --form-id 是否已在 ERP 中启用集成,以及字段名/过滤条件是否正确", 0, data)
	}
	return nil
}

// findResponseStatus looks for a ResponseStatus map, unwrapping up to the first
// element of any array nesting (K3 wraps query errors as [[{...}]]).
func findResponseStatus(v interface{}) map[string]interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		return digResponseStatus(x)
	case []interface{}:
		if len(x) > 0 {
			return findResponseStatus(x[0])
		}
	}
	return nil
}

func digResponseStatus(m map[string]interface{}) map[string]interface{} {
	if rs, ok := m["ResponseStatus"].(map[string]interface{}); ok {
		return rs
	}
	if inner, ok := m["Result"].(map[string]interface{}); ok {
		if rs, ok := inner["ResponseStatus"].(map[string]interface{}); ok {
			return rs
		}
	}
	return nil
}
