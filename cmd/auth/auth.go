// Package auth implements `openerp auth` (test/status) over K3 LoginBySign.
package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
)

// New builds the `auth` command group.
func New(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "鉴权:验证凭据 / 查看 session 状态",
	}
	cmd.AddCommand(newTest(f), newStatus(f))
	return cmd
}

func newTest(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "执行一次 LoginBySign 验证凭据",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := f.Client()
			if err != nil {
				return err
			}
			if err := c.Login(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.Out, "✓ 认证通过 (LoginResultType=1) session=%s\n", c.MaskedSession())
			return nil
		},
	}
}

func newStatus(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "查看当前 profile 的本地 session 状态(不发起请求)",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := f.Config()
			if err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.Out, "profile=%s acctId=%s user=%s server=%s\n",
				p.Name, p.AcctID, p.UserName, p.ServerURL)
			fmt.Fprintln(f.IOStreams.Out, "(运行 `openerp auth test` 验证凭据并刷新 session)")
			return nil
		},
	}
}
