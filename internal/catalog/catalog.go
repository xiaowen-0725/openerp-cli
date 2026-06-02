// Package catalog holds the curated, instance-verified map of business domains →
// query objects (FormId + default fields + filter templates). It is embedded at
// build time (domains.json) and drives the data-driven domain commands, so adding
// a domain/object is a JSON edit, not new Go code.
package catalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

//go:embed domains.json
var domainsJSON []byte

// Catalog is the root of domains.json.
type Catalog struct {
	Domains []Domain `json:"domains"`
}

// Domain is a business area (CLI command group), e.g. "sales".
type Domain struct {
	Name    string   `json:"name"`
	Title   string   `json:"title"`
	Objects []Object `json:"objects"`
}

// Object is one queryable business object (CLI subcommand), e.g. "order".
type Object struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	FormID        string   `json:"formId"`
	NumberField   string   `json:"numberField,omitempty"`
	DefaultFields []string `json:"defaultFields"`
	Filters       []Filter `json:"filters,omitempty"`
	SupportsView  bool     `json:"supportsView,omitempty"`
	Verified      bool     `json:"verified,omitempty"`
	Note          string   `json:"note,omitempty"`
}

// Filter is a named filter template; the flag's value is substituted for "{{.}}".
type Filter struct {
	Flag     string `json:"flag"`
	Template string `json:"template"`
	Required bool   `json:"required,omitempty"`
	Desc     string `json:"desc,omitempty"`
}

var loaded *Catalog

// Load parses the embedded catalog (cached).
func Load() (*Catalog, error) {
	if loaded != nil {
		return loaded, nil
	}
	var c Catalog
	if err := json.Unmarshal(domainsJSON, &c); err != nil {
		return nil, fmt.Errorf("catalog domains.json 解析失败: %w", err)
	}
	loaded = &c
	return loaded, nil
}

// Fields returns the default field list as a comma-joined FieldKeys string.
func (o Object) Fields() string { return strings.Join(o.DefaultFields, ",") }

// RenderFilter builds a K3 FilterString from the object's filter templates and
// the provided flag values. Required filters that are missing produce an error.
// Single quotes in values are rejected to avoid FilterString injection.
func (o Object) RenderFilter(values map[string]string) (string, error) {
	var parts []string
	for _, f := range o.Filters {
		v := strings.TrimSpace(values[f.Flag])
		if v == "" {
			if f.Required {
				return "", fmt.Errorf("缺少必填过滤 --%s（%s）", f.Flag, f.Desc)
			}
			continue
		}
		if strings.Contains(v, "'") {
			return "", fmt.Errorf("--%s 的值不能包含单引号", f.Flag)
		}
		tmpl, err := template.New("f").Parse(f.Template)
		if err != nil {
			return "", fmt.Errorf("过滤模板非法 (%s): %w", f.Flag, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, v); err != nil {
			return "", err
		}
		parts = append(parts, buf.String())
	}
	return strings.Join(parts, " and "), nil
}
