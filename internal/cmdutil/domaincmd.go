package cmdutil

import (
	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/catalog"
	"github.com/xiaowen-0725/openerp-cli/internal/k3client"
)

// DomainCommands builds one cobra command group per catalog domain, each with an
// object subgroup exposing `list` (and `view` where supported). All routed
// through RunBillQuery/RunView, so domains are pure data (domains.json), no
// per-object Go code.
func (f *Factory) DomainCommands() ([]*cobra.Command, error) {
	c, err := catalog.Load()
	if err != nil {
		return nil, err
	}
	var cmds []*cobra.Command
	for _, d := range c.Domains {
		dc := &cobra.Command{Use: d.Name, Short: d.Title}
		for _, obj := range d.Objects {
			dc.AddCommand(f.objectCmd(obj))
		}
		cmds = append(cmds, dc)
	}
	return cmds, nil
}

func (f *Factory) objectCmd(obj catalog.Object) *cobra.Command {
	oc := &cobra.Command{Use: obj.Name, Short: obj.Title}
	oc.AddCommand(f.listCmd(obj))
	if obj.SupportsView && obj.NumberField != "" {
		oc.AddCommand(f.viewCmd(obj))
	}
	return oc
}

func (f *Factory) listCmd(obj catalog.Object) *cobra.Command {
	var (
		top       int
		pageAll   bool
		pageLimit int
		sum       string
		groupBy   string
	)
	vals := map[string]*string{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: obj.Title + " — 列表查询",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fv := map[string]string{}
			for k, p := range vals {
				fv[k] = *p
			}
			filter, err := obj.RenderFilter(fv)
			if err != nil {
				return errs.NewValidation(err.Error(), "查看 --help 的过滤选项", "")
			}
			q := k3client.QueryArgs{FormID: obj.FormID, Fields: obj.Fields(), Filter: filter, Top: top}
			return f.RunBillQuery(cmd.Context(), q, QueryOpts{PageAll: pageAll, PageLimit: pageLimit, Sum: sum, GroupBy: groupBy})
		},
	}
	for _, fl := range obj.Filters {
		desc := fl.Desc
		if fl.Required {
			desc += "（必填）"
		}
		vals[fl.Flag] = cmd.Flags().String(fl.Flag, "", desc)
	}
	cmd.Flags().IntVar(&top, "top", 0, "返回上限 (0=不限)")
	cmd.Flags().BoolVar(&pageAll, "page-all", false, "自动翻页直到取完")
	cmd.Flags().IntVar(&pageLimit, "page-limit", 10, "--page-all 最多翻页数 (0=不限)")
	cmd.Flags().StringVar(&sum, "sum", "", "对该字段求和(自动补进查询列、自动取全量)")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "按该字段分组统计(配合 --sum)")
	return cmd
}

func (f *Factory) viewCmd(obj catalog.Object) *cobra.Command {
	var number string
	cmd := &cobra.Command{
		Use:   "view",
		Short: obj.Title + " — 按编号查看单据",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if number == "" {
				return errs.NewValidation("缺少 --number", "按单据编号/编码查看，如 --number <编号>", "number")
			}
			return f.RunView(cmd.Context(), obj.FormID, number)
		},
	}
	cmd.Flags().StringVar(&number, "number", "", "单据编号/编码")
	return cmd
}
