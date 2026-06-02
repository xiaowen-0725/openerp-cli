package cmdutil

import (
	"context"
	"encoding/json"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/k3client"
	"github.com/xiaowen-0725/openerp-cli/internal/output"
)

// RunBillQuery executes an ExecuteBillQuery, honoring --dry-run and --page-all,
// and emits the result through the standard envelope. Shared by `query` and the
// `bom list` domain command.
func (f *Factory) RunBillQuery(ctx context.Context, q k3client.QueryArgs, pageAll bool, pageLimit int) error {
	c, err := f.Client()
	if err != nil {
		return err
	}
	if f.DryRun {
		p := c.Prepare(k3client.EndpointExecuteBillQuery, k3client.BuildBillQueryParams(q))
		return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, p, nil)
	}
	if pageAll {
		return f.runBillQueryPaged(ctx, c, q, pageLimit)
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

func (f *Factory) runBillQueryPaged(ctx context.Context, c *k3client.Client, q k3client.QueryArgs, pageLimit int) error {
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
			return err
		}
		data := decode(raw)
		if apiErr := apiErrorIfAny(data); apiErr != nil {
			return apiErr
		}
		rows, ok := data.([]interface{})
		if !ok {
			return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, data, nil)
		}
		all = append(all, rows...)
		if len(rows) < pageSize {
			break
		}
		start += len(rows)
	}
	return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, all, &output.Meta{Count: len(all)})
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
