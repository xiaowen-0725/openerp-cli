// Package schema implements `openerp schema <FormId>` — field/metadata discovery
// for any business object via QueryBusinessInfo. Use it to learn an object's real
// field keys (and entries) before building a `query`.
package schema

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
)

// New builds the `schema` command.
func New(f *cmdutil.Factory) *cobra.Command {
	var fieldsOnly bool
	cmd := &cobra.Command{
		Use:   "schema <FormId>",
		Short: "发现某业务对象的可查询字段 (QueryBusinessInfo)",
		Args:  cobra.MaximumNArgs(1),
		Example: `  openerp schema BD_MATERIAL --fields-only
  openerp schema SAL_SaleOrder`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return errs.NewValidation("缺少 FormId",
					"如 `openerp schema SAL_SaleOrder`（可先用 `openerp objects --keyword 销售` 找 FormId）", "")
			}
			return f.RunBusinessInfo(cmd.Context(), args[0], fieldsOnly)
		},
	}
	cmd.Flags().BoolVar(&fieldsOnly, "fields-only", false, "只输出精简字段列表 (key/中文名/类型/关联对象)，便于拷进 query --fields")
	return cmd
}
