// Package config implements `openerp config` (set/list/use/path).
package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
	"github.com/xiaowen-0725/openerp-cli/internal/config"
)

// New builds the `config` command group.
func New(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "管理金蝶云凭据 profile",
	}
	cmd.AddCommand(newSet(f), newList(f), newUse(f), newPath(f))
	return cmd
}

func newSet(f *cmdutil.Factory) *cobra.Command {
	var p config.Profile
	cmd := &cobra.Command{
		Use:   "set",
		Short: "新增或更新一个凭据 profile",
		Example: `  openerp config set --profile prod \
    --server-url "https://akeparking.ik3cloud.com/K3Cloud/" \
    --acct-id 20200817175758703 --user 追溯系统 \
    --app-id <APP_ID> --app-secret <APP_SECRET> --lcid 2052`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if p.Name == "" {
				return errs.NewValidation("缺少 --profile", "为该凭据取个名字,如 --profile prod", "profile")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			// Merge: start from existing profile (if any), override only changed flags.
			merged := config.Profile{Name: p.Name}
			if existing, ok := cfg.Find(p.Name); ok {
				merged = *existing
			}
			fl := cmd.Flags()
			if fl.Changed("server-url") {
				merged.ServerURL = p.ServerURL
			}
			if fl.Changed("acct-id") {
				merged.AcctID = p.AcctID
			}
			if fl.Changed("user") {
				merged.UserName = p.UserName
			}
			if fl.Changed("app-id") {
				merged.AppID = p.AppID
			}
			if fl.Changed("app-secret") {
				merged.AppSecret = p.AppSecret
			}
			if fl.Changed("lcid") {
				merged.LCID = p.LCID
			}
			if merged.LCID == 0 {
				merged.LCID = config.DefaultLCID
			}
			cfg.Upsert(merged)
			if cfg.CurrentProfile == "" {
				cfg.CurrentProfile = merged.Name
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.Out, "已保存 profile %q (当前: %s)\n", merged.Name, cfg.CurrentProfile)
			return nil
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&p.Name, "profile", "", "profile 名 (必填)")
	fl.StringVar(&p.ServerURL, "server-url", "", "K3Cloud 服务地址,以 /K3Cloud/ 结尾")
	fl.StringVar(&p.AcctID, "acct-id", "", "数据中心 ID (acctId)")
	fl.StringVar(&p.UserName, "user", "", "登录用户名")
	fl.StringVar(&p.AppID, "app-id", "", "第三方应用 appId")
	fl.StringVar(&p.AppSecret, "app-secret", "", "第三方应用 appSecret")
	fl.IntVar(&p.LCID, "lcid", config.DefaultLCID, "语言 LCID (默认 2052)")
	return cmd
}

func newList(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出所有 profile (appSecret 掩码)",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 {
				fmt.Fprintln(f.IOStreams.Out, "(无 profile) 用 `openerp config set` 新建")
				return nil
			}
			for _, p := range cfg.Profiles {
				marker := " "
				if p.Name == cfg.CurrentProfile {
					marker = "*"
				}
				fmt.Fprintf(f.IOStreams.Out, "%s %-12s server=%s acct=%s user=%s appId=%s secret=%s lcid=%d\n",
					marker, p.Name, p.ServerURL, p.AcctID, p.UserName, p.AppID, config.Mask(p.AppSecret), p.LCID)
			}
			return nil
		},
	}
}

func newUse(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "切换当前 profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Find(args[0]); !ok {
				return errs.NewValidation(fmt.Sprintf("profile %q 不存在", args[0]), "用 `openerp config list` 查看", "profile")
			}
			cfg.CurrentProfile = args[0]
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.Out, "当前 profile: %s\n", args[0])
			return nil
		},
	}
}

func newPath(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "打印配置文件路径",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			fmt.Fprintln(f.IOStreams.Out, p)
			return nil
		},
	}
}
