// Package objects implements `openerp objects` — business-object discovery by
// querying BOS_ObjectType (the system's catalog of all forms). The discovery
// backbone: find a FormId, then `schema` it, then `query` it.
package objects

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
	"github.com/xiaowen-0725/openerp-cli/internal/k3client"
)

const objectTypeFields = "FID,FName,FSubSystemId"

// New builds the `objects` command.
func New(f *cmdutil.Factory) *cobra.Command {
	var (
		keyword   string
		subsystem string
		top       int
		pageAll   bool
		pageLimit int
	)
	cmd := &cobra.Command{
		Use:   "objects",
		Short: "发现业务对象：搜索系统全部 FormId (查 BOS_ObjectType)",
		Long: `搜索金蝶系统中的业务对象，拿到 FormId，供 schema/query 使用。
发现工作流：objects --keyword <词> → schema <FormId> → query --form-id <FormId>。`,
		Example: `  openerp objects --keyword 销售订单
  openerp objects --keyword 委外 --top 30`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(keyword) == "" && strings.TrimSpace(subsystem) == "" {
				return errs.NewValidation("objects 需要至少一个收敛条件",
					"加 --keyword <关键词> 或 --subsystem <子系统>（BOS_ObjectType 数据量大，需收敛）", "keyword")
			}
			var conds []string
			if keyword != "" {
				if strings.Contains(keyword, "'") {
					return errs.NewValidation("--keyword 不能包含单引号", "去掉引号重试", "keyword")
				}
				conds = append(conds, "FName like '%"+keyword+"%'")
			}
			if subsystem != "" {
				if strings.Contains(subsystem, "'") {
					return errs.NewValidation("--subsystem 不能包含单引号", "去掉引号重试", "subsystem")
				}
				conds = append(conds, "FSubSystemId like '%"+subsystem+"%'")
			}
			q := k3client.QueryArgs{
				FormID: "BOS_ObjectType",
				Fields: objectTypeFields,
				Filter: strings.Join(conds, " and "),
				Top:    top,
			}
			return f.RunBillQuery(cmd.Context(), q, pageAll, pageLimit)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&keyword, "keyword", "", "按名称模糊搜索 (FName like)")
	fl.StringVar(&subsystem, "subsystem", "", "按子系统过滤 (FSubSystemId)")
	fl.IntVar(&top, "top", 50, "返回上限 (默认 50)")
	fl.BoolVar(&pageAll, "page-all", false, "自动翻页直到取完")
	fl.IntVar(&pageLimit, "page-limit", 10, "--page-all 最多翻页数 (0=不限)")
	return cmd
}
