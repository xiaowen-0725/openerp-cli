// Package bom implements `openerp bom` (view/list) — an ergonomic domain
// command over ENG_BOM that a skill can wrap, so agents needn't know dot-path
// field names. Demonstrates the domain-command UX layer.
package bom

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zhoujw/openerp-cli/errs"
	"github.com/zhoujw/openerp-cli/internal/cmdutil"
	"github.com/zhoujw/openerp-cli/internal/k3client"
)

// formBOM is the K3 engineering BOM form.
const formBOM = "ENG_BOM"

// bomChildFields is the verified BOM child-item field set (from the Python anchor).
const bomChildFields = "FMATERIALIDCHILD.FNumber,FMATERIALIDCHILD.FName,FMATERIALIDCHILD.FSpecification," +
	"FNumerator,FDenominator,FScrapRate,FMATERIALIDCHILD.FErpClsID,FUnitID.FName,FNumber"

// New builds the `bom` command group.
func New(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bom",
		Short: "BOM (物料清单) 查询",
	}
	cmd.AddCommand(newView(f), newList(f))
	return cmd
}

func newView(f *cmdutil.Factory) *cobra.Command {
	var number string
	cmd := &cobra.Command{
		Use:     "view",
		Short:   "按编号查看单个 BOM (View)",
		Example: `  openerp bom view --number "1.30.67.0132 VA0"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if number == "" {
				return errs.NewValidation("缺少 --number", `如 --number "1.30.67.0132 VA0"`, "number")
			}
			return f.RunView(cmd.Context(), formBOM, number)
		},
	}
	cmd.Flags().StringVar(&number, "number", "", "BOM 编号 (必填)")
	return cmd
}

func newList(f *cmdutil.Factory) *cobra.Command {
	var (
		material string
		top      int
	)
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "按父物料列出 BOM 子项 (ExecuteBillQuery)",
		Example: `  openerp bom list --material "1.30.67.0132" --top 100`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if material == "" {
				return errs.NewValidation("缺少 --material", "父物料编码,如 --material 1.30.67.0132", "material")
			}
			q := k3client.QueryArgs{
				FormID: formBOM,
				Fields: bomChildFields,
				Filter: fmt.Sprintf("FMATERIALID.FNumber='%s'", material),
				Top:    top,
			}
			return f.RunBillQuery(cmd.Context(), q, false, 0)
		},
	}
	cmd.Flags().StringVar(&material, "material", "", "父物料编码 (必填)")
	cmd.Flags().IntVar(&top, "top", 0, "返回上限 (0=不限)")
	return cmd
}
