// Package cmd assembles the openerp root command and dispatches errors through
// the typed-error → exit-code contract.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	attachmentcmd "github.com/xiaowen-0725/openerp-cli/cmd/attachment"
	authcmd "github.com/xiaowen-0725/openerp-cli/cmd/auth"
	bomcmd "github.com/xiaowen-0725/openerp-cli/cmd/bom"
	configcmd "github.com/xiaowen-0725/openerp-cli/cmd/config"
	doctorcmd "github.com/xiaowen-0725/openerp-cli/cmd/doctor"
	objectscmd "github.com/xiaowen-0725/openerp-cli/cmd/objects"
	querycmd "github.com/xiaowen-0725/openerp-cli/cmd/query"
	schemacmd "github.com/xiaowen-0725/openerp-cli/cmd/schema"
	updatecmd "github.com/xiaowen-0725/openerp-cli/cmd/update"
	versionpkg "github.com/xiaowen-0725/openerp-cli/cmd/version"
	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
	"github.com/xiaowen-0725/openerp-cli/internal/output"
	"github.com/xiaowen-0725/openerp-cli/internal/selfupdate"
)

// Version is the CLI version. Source of truth lives in cmd/version (a leaf
// package the self-updater can import without a cycle); kept here for any
// existing reference.
var Version = versionpkg.Version

// NewRootCmd builds the root command and binds global flags onto f.
func NewRootCmd(f *cmdutil.Factory) *cobra.Command {
	root := &cobra.Command{
		Use:           "openerp",
		Short:         "金蝶云·星空 ERP CLI —— 为人和 AI Agent 而生",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Throttled, silent background self-update (once/day). Skips the updater's
		// own commands so it never recurses; honors --no-update-check / OPENERP_NO_UPDATE.
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			if f.NoUpdateCheck || isUpdaterCommand(c) {
				return
			}
			selfupdate.MaybeBackgroundUpdate(Version)
		},
		Long: `openerp 把金蝶云·星空(K3 Cloud)的接口封装成命令行工具。
本轮为只读 POC：自动处理 LoginBySign 鉴权与 session 复用,多 profile 凭据管理。

常用:
  openerp config set --profile prod --server-url ... --acct-id ... --user ... --app-id ... --app-secret ...
  openerp doctor                                       # 自检:配置/登录/查询
  openerp auth test                                    # 验证凭据
  openerp query --form-id ENG_BOM --fields "FNumber" --filter "FMATERIALID.FNumber='1.30.67.0132'"`,
	}

	pf := root.PersistentFlags()
	pf.StringVar(&f.Profile, "profile", "", "凭据 profile 名(默认当前 profile)")
	pf.StringVar(&f.Env, "env", "", "环境(预留;星空实例 URL 随 profile 固定)")
	pf.StringVar(&f.Format, "format", "json", "输出格式 json|ndjson|table|csv")
	pf.StringVarP(&f.Jq, "jq", "q", "", "jq 路径子集(如 .data / .data[0] / .data.FName)")
	pf.BoolVar(&f.DryRun, "dry-run", false, "只打印将发送的请求,不实际发送")
	pf.BoolVarP(&f.Verbose, "verbose", "v", false, "调试日志输出到 stderr")
	pf.BoolVar(&f.ReadOnly, "read-only", false, "只读模式(本轮命令均只读)")
	pf.BoolVar(&f.NoUpdateCheck, "no-update-check", false, "禁用本次运行的自动更新检查(等价 OPENERP_NO_UPDATE=1)")

	root.AddCommand(
		configcmd.New(f),
		authcmd.New(f),
		doctorcmd.New(f),
		objectscmd.New(f),
		schemacmd.New(f),
		querycmd.New(f),
		bomcmd.New(f),
		attachmentcmd.New(f),
		updatecmd.New(f),
		updatecmd.NewHidden(f),
	)
	// Catalog-driven domain command groups (base/purchase/sales/inventory/...).
	if domainCmds, err := f.DomainCommands(); err == nil {
		root.AddCommand(domainCmds...)
	}

	// Pure groups (no Run) should error on a missing/unknown subcommand instead
	// of silently printing help (larksuite/cli pattern). Root keeps cobra's
	// default help-on-no-args behavior.
	for _, child := range root.Commands() {
		installUnknownSubcommandGuard(child)
	}
	return root
}

// isUpdaterCommand reports whether c is the explicit/hidden update path, where a
// background self-update check would be redundant or recursive.
func isUpdaterCommand(c *cobra.Command) bool {
	switch c.Name() {
	case "update", "__selfupdate-apply", "version", "help":
		return true
	}
	return false
}

func installUnknownSubcommandGuard(cmd *cobra.Command) {
	if cmd.HasSubCommands() && cmd.Run == nil && cmd.RunE == nil {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errs.NewValidation(fmt.Sprintf("%q 需要一个子命令", c.CommandPath()),
					"运行 `"+c.CommandPath()+" --help` 查看可用子命令", "")
			}
			return errs.NewValidation(fmt.Sprintf("%s 下没有子命令 %q", c.CommandPath(), args[0]),
				"运行 `"+c.CommandPath()+" --help` 查看可用子命令", "")
		}
	}
	for _, ch := range cmd.Commands() {
		installUnknownSubcommandGuard(ch)
	}
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	f := cmdutil.NewFactory()
	root := NewRootCmd(f)
	err := root.Execute()
	if err == nil {
		return output.ExitOK
	}
	return output.EmitError(f.IOStreams.Err, err)
}
