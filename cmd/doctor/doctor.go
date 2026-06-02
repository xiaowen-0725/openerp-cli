// Package doctor implements `openerp doctor` — a sequential self-check
// (config → login → probe query) that emits a structured result and a
// health-reflecting exit code. A strong entry point for agents.
package doctor

import (
	"github.com/spf13/cobra"

	"github.com/zhoujw/openerp-cli/errs"
	"github.com/zhoujw/openerp-cli/internal/cmdutil"
	"github.com/zhoujw/openerp-cli/internal/k3client"
	"github.com/zhoujw/openerp-cli/internal/output"
)

type check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// New builds the `doctor` command.
func New(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "自检: 配置完整性 → 登录 → 一次探测查询",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var checks []check

			// 1) config
			p, err := f.Config()
			if err != nil {
				checks = append(checks, fail("config", err))
				return emit(f, checks, err)
			}
			checks = append(checks, check{Name: "config", OK: true, Detail: "profile=" + p.Name})

			// 2) login (LoginBySign)
			c, err := f.Client()
			if err != nil {
				checks = append(checks, fail("login", err))
				return emit(f, checks, err)
			}
			if err := c.Login(ctx); err != nil {
				checks = append(checks, fail("login", err))
				return emit(f, checks, err)
			}
			checks = append(checks, check{Name: "login", OK: true, Detail: "session=" + c.MaskedSession()})

			// 3) probe query (ENG_BOM top 1)
			if _, err := c.ExecuteBillQuery(ctx, k3client.QueryArgs{FormID: "ENG_BOM", Fields: "FNumber", Top: 1}); err != nil {
				checks = append(checks, fail("probe", err))
				return emit(f, checks, err)
			}
			checks = append(checks, check{Name: "probe", OK: true, Detail: "ENG_BOM top 1 ok"})

			return emit(f, checks, nil)
		},
	}
}

func fail(name string, err error) check {
	return check{Name: name, OK: false, Detail: err.Error(), Hint: hintOf(err)}
}

func hintOf(err error) string {
	if p, ok := errs.ProblemOf(err); ok {
		return p.Hint
	}
	return ""
}

// emit prints {ok, checks} to stdout and returns a SilentExit (no double print)
// carrying the failure's exit code, or nil when all checks passed.
func emit(f *cmdutil.Factory, checks []check, failErr error) error {
	_ = output.PrintJSON(f.IOStreams.Out, map[string]interface{}{
		"ok":     failErr == nil,
		"checks": checks,
	})
	if failErr != nil {
		return output.SilentExit{Code: output.ExitCodeOf(failErr)}
	}
	return nil
}
