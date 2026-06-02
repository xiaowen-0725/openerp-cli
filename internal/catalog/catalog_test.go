package catalog

import (
	"strings"
	"testing"
)

func TestLoadStructure(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Domains) == 0 {
		t.Fatal("no domains")
	}
	seenDomain := map[string]bool{}
	for _, d := range c.Domains {
		if d.Name == "" {
			t.Error("domain missing name")
		}
		if seenDomain[d.Name] {
			t.Errorf("duplicate domain %q", d.Name)
		}
		seenDomain[d.Name] = true
		if len(d.Objects) == 0 {
			t.Errorf("domain %q has no objects", d.Name)
		}
		seenObj := map[string]bool{}
		for _, o := range d.Objects {
			if o.Name == "" || o.FormID == "" {
				t.Errorf("%s: object missing name/formId", d.Name)
			}
			if seenObj[o.Name] {
				t.Errorf("%s: duplicate object %q", d.Name, o.Name)
			}
			seenObj[o.Name] = true
			if len(o.DefaultFields) == 0 {
				t.Errorf("%s/%s: no defaultFields", d.Name, o.Name)
			}
			if o.SupportsView && o.NumberField == "" {
				t.Errorf("%s/%s: supportsView but no numberField", d.Name, o.Name)
			}
			seenFlag := map[string]bool{}
			for _, fl := range o.Filters {
				if fl.Flag == "" || fl.Template == "" {
					t.Errorf("%s/%s: filter missing flag/template", d.Name, o.Name)
				}
				if seenFlag[fl.Flag] {
					t.Errorf("%s/%s: duplicate filter flag %q", d.Name, o.Name, fl.Flag)
				}
				seenFlag[fl.Flag] = true
				if !strings.Contains(fl.Template, "{{.}}") {
					t.Errorf("%s/%s: filter %q template missing {{.}}", d.Name, o.Name, fl.Flag)
				}
			}
		}
	}
}

func TestRenderFilter(t *testing.T) {
	o := Object{Filters: []Filter{
		{Flag: "bill-no", Template: "FBillNo='{{.}}'"},
		{Flag: "date-from", Template: "FDate>='{{.}}'"},
		{Flag: "req", Template: "FX='{{.}}'", Required: true, Desc: "必填项"},
	}}
	if _, err := o.RenderFilter(map[string]string{"bill-no": "A"}); err == nil {
		t.Error("expected error for missing required filter")
	}
	s, err := o.RenderFilter(map[string]string{"bill-no": "A", "date-from": "2024-01-01", "req": "R"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"FBillNo='A'", "FDate>='2024-01-01'", "FX='R'", " and "} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered %q missing %q", s, want)
		}
	}
	if _, err := o.RenderFilter(map[string]string{"req": "a'b"}); err == nil {
		t.Error("expected error for single-quote injection")
	}
}
