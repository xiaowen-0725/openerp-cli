package cmdutil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// resolveOutPath decides the final file path for a downloaded attachment.
// outFlag empty  → "./<serverFileName>"
// outFlag is dir → "<dir>/<serverFileName>"
// outFlag is file→ outFlag as-is
// If the target exists and overwrite is false, returns a *ValidationError so the
// user gets an actionable hint (exit 2).
func resolveOutPath(outFlag, serverFileName string, overwrite bool) (string, error) {
	path := "./" + serverFileName
	if outFlag != "" {
		if info, err := os.Stat(outFlag); err == nil && info.IsDir() {
			path = filepath.Join(outFlag, serverFileName)
		} else {
			path = outFlag
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errs.NewValidation("无法解析输出路径: "+err.Error(), "检查 --out 是否合法", "out")
	}
	if !overwrite {
		if _, err := os.Stat(abs); err == nil {
			return "", errs.NewValidation("目标文件已存在: "+abs,
				"加 --overwrite 覆盖,或换一个 --out 路径", "out")
		}
	}
	return abs, nil
}

// RunAttachmentDownLoad fetches all chunks of an attachment and writes them to a
// local file. It loops AttachmentDownLoadChunk until IsLast, decoding each base64
// FilePart into the output file. Progress is written to stderr (human-readable);
// on success a small JSON result envelope goes to stdout (machine-readable). This
// split honors the stdout=data / stderr=everything-else contract. On any failure
// mid-stream the partial file is closed and removed so no truncated file remains.
func (f *Factory) RunAttachmentDownLoad(ctx context.Context, fileID, outFlag string, overwrite bool) error {
	c, err := f.Client()
	if err != nil {
		return err
	}

	// dry-run prints only the first chunk's request (no login, no send).
	if f.DryRun {
		p := c.Prepare(k3client.EndpointAttachmentDownLoad, k3client.BuildAttachmentDownLoadParams(fileID, 0))
		return output.EmitData(f.IOStreams.Out, f.Format, f.Jq, p, nil)
	}

	var (
		file       *os.File
		path       string
		serverName string
		total      int64
		chunks     int
	)
	start := int64(0)
	for {
		r, err := c.AttachmentDownLoadChunk(ctx, fileID, start)
		if err != nil {
			if file != nil {
				file.Close()
				_ = os.Remove(path) // clean up the partial file
				fmt.Fprintf(f.IOStreams.Err, "下载失败,已删除未完成的文件: %s\n", path)
			}
			return err
		}
		// First chunk: lock the output path (server FileName) and open the file.
		if chunks == 0 {
			serverName = r.FileName
			path, err = resolveOutPath(outFlag, r.FileName, overwrite)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return errs.NewValidation("无法创建输出目录: "+err.Error(), "检查 --out 所在目录权限", "out")
			}
			file, err = os.Create(path)
			if err != nil {
				return errs.NewValidation("无法创建输出文件: "+err.Error(), "检查 --out 路径权限", "out")
			}
		}
		// Decode this chunk's base64 payload and append to the file.
		part, derr := base64.StdEncoding.DecodeString(r.FilePart)
		if derr != nil {
			file.Close()
			_ = os.Remove(path)
			fmt.Fprintf(f.IOStreams.Err, "base64 解码失败,已删除未完成的文件: %s\n", path)
			return errs.NewAPI("附件块 base64 解码失败: "+derr.Error(), "", 0, nil)
		}
		if _, werr := file.Write(part); werr != nil {
			file.Close()
			_ = os.Remove(path)
			fmt.Fprintf(f.IOStreams.Err, "写文件失败,已删除未完成的文件: %s\n", path)
			return errs.NewValidation("写文件失败: "+werr.Error(), "检查磁盘空间与 --out 路径权限", "out")
		}
		total += int64(len(part))
		chunks++
		fmt.Fprintf(f.IOStreams.Err, "[%d] %s 已下载 %d/%d bytes\n", chunks, r.FileName, total, r.FileSize)
		if r.IsLast {
			break
		}
		start = r.StartIndex
	}
	if file != nil {
		if err := file.Close(); err != nil {
			return errs.NewValidation("关闭文件失败: "+err.Error(), "", "out")
		}
	}

	// stdout: small result envelope so an agent can confirm the download.
	return output.EmitData(f.IOStreams.Out, f.Format, f.Jq,
		map[string]any{
			"fileName": serverName,
			"path":     path,
			"size":     total,
			"chunks":   chunks,
		}, nil)
}
