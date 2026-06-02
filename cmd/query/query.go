// Package query implements `openerp query` — a generic ExecuteBillQuery
// passthrough (the universal read path), modeled on larksuite/cli's `api`.
package query

import (
	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
	"github.com/xiaowen-0725/openerp-cli/internal/k3client"
)

// New builds the `query` command.
func New(f *cmdutil.Factory) *cobra.Command {
	var (
		q         k3client.QueryArgs
		pageAll   bool
		pageLimit int
		sum       string
		groupBy   string
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "通用查询 (ExecuteBillQuery)",
		Long: `对任意业务对象执行 ExecuteBillQuery 列表查询。

示例:
  openerp query --form-id ENG_BOM \
    --fields "FMATERIALIDCHILD.FNumber,FMATERIALIDCHILD.FName,FNumerator" \
    --filter "FMATERIALID.FNumber='1.30.67.0132'" --top 100`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if q.FormID == "" {
				return errs.NewValidation("缺少 --form-id", "如 --form-id ENG_BOM / PUR_PriceCategory", "form-id")
			}
			if q.Fields == "" {
				return errs.NewValidation("缺少 --fields", "逗号分隔字段,支持点号,如 FMATERIALIDCHILD.FNumber", "fields")
			}
			return f.RunBillQuery(cmd.Context(), q, cmdutil.QueryOpts{PageAll: pageAll, PageLimit: pageLimit, Sum: sum, GroupBy: groupBy})
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&q.FormID, "form-id", "", "业务对象 FormId (必填)")
	fl.StringVar(&q.Fields, "fields", "", "返回字段,逗号分隔 (必填)")
	fl.StringVar(&q.Filter, "filter", "", "过滤条件 (SQL 风格),如 FMATERIALID.FNumber='...'")
	fl.StringVar(&q.Order, "order", "", "排序,如 FNumber desc")
	fl.IntVar(&q.Top, "top", 0, "TopRowCount 上限 (0=不限)")
	fl.IntVar(&q.Start, "start", 0, "StartRow 起始行")
	fl.IntVar(&q.Limit, "limit", 0, "单次返回行数 Limit (0=服务端默认)")
	fl.BoolVar(&pageAll, "page-all", false, "自动翻页直到取完")
	fl.IntVar(&pageLimit, "page-limit", 10, "--page-all 最多翻页数 (0=不限)")
	fl.StringVar(&sum, "sum", "", "对该字段求和(自动补进 --fields、自动取全量)")
	fl.StringVar(&groupBy, "group-by", "", "按该字段分组统计(配合 --sum)")
	return cmd
}
