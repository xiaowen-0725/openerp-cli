package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xiaowen-0725/openerp-cli/errs"
)

// PendingNotice, if set, returns the system "_notice" map to inject into success
// envelopes (reserved for update/skills drift; nil for the POC).
var PendingNotice func() map[string]interface{}

func notice() map[string]interface{} {
	if PendingNotice == nil {
		return nil
	}
	return PendingNotice()
}

// SilentExit carries an exit code whose envelope has already been written by the
// command itself (e.g. doctor). EmitError prints nothing and returns the code.
type SilentExit struct{ Code int }

func (s SilentExit) Error() string { return "" }

// EmitData wraps data in the success envelope and renders it to out in the
// requested format. When jqExpr is non-empty (and not "."), the envelope is
// filtered by the jq-path subset and the selected value is printed as JSON
// (format is ignored in that case).
func EmitData(out io.Writer, format, jqExpr string, data interface{}, meta *Meta) error {
	env := Envelope{OK: true, Data: data, Meta: meta, Notice: notice()}

	if e := strings.TrimSpace(jqExpr); e != "" && e != "." {
		raw, err := json.Marshal(env)
		if err != nil {
			return errs.NewValidation("无法序列化结果用于 --jq", "去掉 --jq 重试", "jq")
		}
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		v, err := applyJQPath(generic, e)
		if err != nil {
			return err
		}
		return printJSON(out, v)
	}

	switch format {
	case "", "json":
		return printJSON(out, env)
	case "ndjson":
		return printNDJSON(out, data)
	case "table":
		return printTable(out, data)
	case "csv":
		return printCSV(out, data)
	default:
		return errs.NewValidation("未知 --format: "+format, "可选 json|ndjson|table|csv", "format")
	}
}

// EmitError writes the typed error envelope to errOut and returns the exit code.
// A SilentExit prints nothing (the command already emitted its own output).
func EmitError(errOut io.Writer, err error) int {
	if err == nil {
		return ExitOK
	}
	var se SilentExit
	if errors.As(err, &se) {
		return se.Code
	}
	payload := errorPayload(err)
	b, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Fprintln(errOut, string(b))
	return ExitCodeOf(err)
}

// errorPayload renders an error as a flat JSON object: {ok:false, type, message,
// hint, ...typed-extension-fields}. Typed errs.* marshal their Problem fields +
// extensions; untyped errors degrade to an internal error.
func errorPayload(err error) map[string]interface{} {
	m := map[string]interface{}{"ok": false}
	if _, ok := errs.ProblemOf(err); ok {
		if raw, e := json.Marshal(err); e == nil {
			var fields map[string]interface{}
			if json.Unmarshal(raw, &fields) == nil {
				for k, v := range fields {
					m[k] = v
				}
				return m
			}
		}
	}
	m["type"] = string(errs.CategoryInternal)
	m["message"] = err.Error()
	return m
}

// PrintJSON pretty-prints v as JSON + newline. Used by diagnostics (e.g. doctor)
// that build their own object and bypass the success envelope.
func PrintJSON(out io.Writer, v interface{}) error { return printJSON(out, v) }

func printJSON(out io.Writer, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errs.NewValidation("结果无法序列化为 JSON", "", "")
	}
	_, err = fmt.Fprintln(out, string(b))
	return err
}

// rowsOf best-effort coerces data into a slice for ndjson/table/csv. Returns
// false when data is not a JSON array (callers fall back to JSON).
func rowsOf(data interface{}) ([]interface{}, bool) {
	switch v := data.(type) {
	case []interface{}:
		return v, true
	case json.RawMessage:
		var arr []interface{}
		if json.Unmarshal(v, &arr) == nil {
			return arr, true
		}
	case []byte:
		var arr []interface{}
		if json.Unmarshal(v, &arr) == nil {
			return arr, true
		}
	}
	return nil, false
}

func printNDJSON(out io.Writer, data interface{}) error {
	rows, ok := rowsOf(data)
	if !ok {
		return printJSON(out, data)
	}
	for _, r := range rows {
		b, _ := json.Marshal(r)
		if _, err := fmt.Fprintln(out, string(b)); err != nil {
			return err
		}
	}
	return nil
}

func cellString(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// rowCells turns one row into string cells. ExecuteBillQuery rows are arrays;
// object rows are flattened to their values (best effort).
func rowCells(row interface{}) []string {
	switch r := row.(type) {
	case []interface{}:
		out := make([]string, len(r))
		for i, c := range r {
			out[i] = cellString(c)
		}
		return out
	default:
		return []string{cellString(r)}
	}
}

func printTable(out io.Writer, data interface{}) error {
	rows, ok := rowsOf(data)
	if !ok {
		return printJSON(out, data)
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(out, strings.Join(rowCells(row), "\t")); err != nil {
			return err
		}
	}
	return nil
}

func printCSV(out io.Writer, data interface{}) error {
	rows, ok := rowsOf(data)
	if !ok {
		return printJSON(out, data)
	}
	w := csv.NewWriter(out)
	for _, row := range rows {
		if err := w.Write(rowCells(row)); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func jqErr(expr string) error {
	return errs.NewValidation("无法对结果求值 jq 路径: "+expr,
		"本 CLI 仅支持 jq 子集: . / .key / .key.sub / .key[N]", "jq")
}

func isIdentChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// applyJQPath evaluates a small jq-path subset over a decoded JSON value:
// ".", ".key", ".key.sub", ".key[N]", ".[N]". Not full jq.
func applyJQPath(v interface{}, expr string) (interface{}, error) {
	if !strings.HasPrefix(expr, ".") {
		return nil, jqErr(expr)
	}
	cur := v
	rest := expr
	for len(rest) > 0 {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			j := 0
			for j < len(rest) && isIdentChar(rest[j]) {
				j++
			}
			key := rest[:j]
			rest = rest[j:]
			if key == "" {
				continue // ".[N]" or trailing "."
			}
			m, ok := cur.(map[string]interface{})
			if !ok {
				return nil, jqErr(expr)
			}
			cur = m[key]
		case '[':
			k := strings.IndexByte(rest, ']')
			if k < 0 {
				return nil, jqErr(expr)
			}
			idx, err := strconv.Atoi(strings.TrimSpace(rest[1:k]))
			if err != nil {
				return nil, jqErr(expr)
			}
			rest = rest[k+1:]
			arr, ok := cur.([]interface{})
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, jqErr(expr)
			}
			cur = arr[idx]
		default:
			return nil, jqErr(expr)
		}
	}
	return cur, nil
}
