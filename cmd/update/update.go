// Package update implements `openerp update` (explicit, human/agent-driven
// upgrade) and the hidden `__selfupdate-apply` invoked by the silent background
// updater. Both share internal/selfupdate. stdout stays a clean JSON envelope;
// human-facing progress goes to stderr.
package update

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/cmd/version"
	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
	"github.com/xiaowen-0725/openerp-cli/internal/output"
	"github.com/xiaowen-0725/openerp-cli/internal/selfupdate"
)

// New builds the `update` command.
func New(f *cmdutil.Factory) *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "检查并更新到最新版 (来源: GitHub Releases)",
		Long: `检查 GitHub Releases 是否有更新, 校验 SHA256 后原子替换当前二进制。

  openerp update            # 有新版则更新
  openerp update --check    # 只检查, 不更新

提示: 通常无需手动运行 —— openerp 会每天静默自检并在后台自动更新,
下次启动即生效。设 OPENERP_NO_UPDATE=1 可关闭自动更新。`,
		RunE: func(c *cobra.Command, _ []string) error {
			return run(c.Context(), f, checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "只检查最新版, 不执行更新")
	return cmd
}

func run(ctx context.Context, f *cmdutil.Factory, checkOnly bool) error {
	cur := version.Version
	fmt.Fprintln(f.IOStreams.Err, "检查最新版本…")
	latest, err := selfupdate.LatestVersion(ctx)
	if err != nil {
		return errs.NewNetwork("无法获取最新版本: "+err.Error(), "检查网络连接, 或访问 https://github.com/"+selfupdate.Repo+"/releases", "conn")
	}

	if !selfupdate.Newer(cur, latest) {
		_ = output.PrintJSON(f.IOStreams.Out, map[string]any{
			"ok": true, "updated": false, "current": cur, "latest": latest,
			"message": "已是最新版本",
		})
		return nil
	}

	if checkOnly {
		fmt.Fprintf(f.IOStreams.Err, "有新版本: %s → %s\n", cur, latest)
		_ = output.PrintJSON(f.IOStreams.Out, map[string]any{
			"ok": true, "updated": false, "current": cur, "latest": latest,
			"update_available": true,
			"message":          "运行 `openerp update` 更新",
		})
		return nil
	}

	if selfupdate.IsDevVersion(cur) {
		return errs.NewValidation("当前为开发构建("+cur+"), 不执行自更新", "请用发布版二进制, 或 `npm i -g @openydt/openerp-cli@latest`", "")
	}

	fmt.Fprintf(f.IOStreams.Err, "更新中: %s → %s …\n", cur, latest)
	res, err := selfupdate.Apply(ctx, cur, latest)
	if err != nil {
		if err == selfupdate.ErrPermission {
			return errs.NewConfig("无权限替换二进制(可能是全局安装目录)",
				"请用 `npm i -g @openydt/openerp-cli@latest` 升级, 或对安装目录授权后重试", "")
		}
		return errs.NewNetwork("更新失败: "+err.Error(), "稍后重试, 或手动 `npm i -g @openydt/openerp-cli@latest`", "conn")
	}

	fmt.Fprintf(f.IOStreams.Err, "已更新到 %s ✓\n", res.To)
	_ = output.PrintJSON(f.IOStreams.Out, map[string]any{
		"ok": true, "updated": true, "from": res.From, "to": res.To, "path": res.Path,
	})
	return nil
}

// NewHidden builds the hidden `__selfupdate-apply <version>` command run by the
// detached background updater. It is fully silent on stdout/stderr (output goes
// to update.log via the parent's redirection) and always exits 0.
func NewHidden(_ *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:    "__selfupdate-apply <version>",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cur := version.Version
			target := args[0]
			if selfupdate.IsDevVersion(cur) || !selfupdate.Newer(cur, target) {
				return nil
			}
			if res, err := selfupdate.Apply(c.Context(), cur, target); err != nil {
				fmt.Printf("[selfupdate] 失败 %s → %s: %v\n", cur, target, err)
			} else {
				fmt.Printf("[selfupdate] 成功 %s → %s (%s)\n", res.From, res.To, res.Path)
			}
			return nil // never fail the detached process
		},
	}
}
