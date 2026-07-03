// Package attachment implements `openerp attachment` (list/get) — attachment
// query and download. `list` is a thin wrapper over query BOS_Attachment (any
// business bill's attachments: FBillNo holds the bill number, FBillType its
// FormId); `get` downloads a file by FileId via the AttachmentDownLoad API,
// chunking at 1MB until IsLast. This is the CLI's first binary-producing command.
package attachment

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/xiaowen-0725/openerp-cli/errs"
	"github.com/xiaowen-0725/openerp-cli/internal/cmdutil"
	"github.com/xiaowen-0725/openerp-cli/internal/k3client"
)

// formAttachment is the K3 attachment-panel form (table T_BAS_ATTACHMENT).
const formAttachment = "BOS_Attachment"

// attachFields is the verified attachment field set (probed against the live
// instance). FFileId feeds `get`; FBillNo/FBillType are the bill cross-reference.
const attachFields = "FAttachmentName,FExtName,FAttachmentSize,FFileId,FFileStorage," +
	"FIsAllowDownLoad,FaliasFileName,FBillNo,FBillType,FModifyTime"

// New builds the `attachment` command group.
func New(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachment",
		Short: "附件查询与下载",
	}
	cmd.AddCommand(newList(f), newGet(f))
	return cmd
}

func newList(f *cmdutil.Factory) *cobra.Command {
	var (
		billNo   string
		billType string
		top      int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "按业务单据编号列出附件 (ExecuteBillQuery on BOS_Attachment)",
		Example: `  openerp attachment list --bill-no 1.20.03.0007
  openerp attachment list --bill-no 1.20.03.0007 --bill-type BD_MATERIAL --top 50`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if billNo == "" {
				return errs.NewValidation("缺少 --bill-no", "业务单据编号,如物料编码 1.20.03.0007", "bill-no")
			}
			filter := fmt.Sprintf("FBillNo='%s'", billNo)
			if billType != "" {
				filter += fmt.Sprintf(" && FBillType='%s'", billType)
			}
			q := k3client.QueryArgs{
				FormID: formAttachment,
				Fields: attachFields,
				Filter: filter,
				Top:    top,
			}
			return f.RunBillQuery(cmd.Context(), q, cmdutil.QueryOpts{})
		},
	}
	cmd.Flags().StringVar(&billNo, "bill-no", "", "业务单据编号 (必填,如物料编码)")
	cmd.Flags().StringVar(&billType, "bill-type", "", "业务对象 FormId (可选,精确过滤,如 BD_MATERIAL)")
	cmd.Flags().IntVar(&top, "top", 0, "返回上限 (0=不限)")
	return cmd
}

func newGet(f *cmdutil.Factory) *cobra.Command {
	var (
		fileID    string
		out       string
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "按 FileId 下载附件 (AttachmentDownLoad, 分块 1MB)",
		Example: `  openerp attachment get --file-id 99f4cb8338af45c6a97996d7d587e105
  openerp attachment get --file-id 99f4... --out ./downloads/ --overwrite`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if fileID == "" {
				return errs.NewValidation("缺少 --file-id", "先用 `openerp attachment list` 拿到 FFileId", "file-id")
			}
			return f.RunAttachmentDownLoad(cmd.Context(), fileID, out, overwrite)
		},
	}
	cmd.Flags().StringVar(&fileID, "file-id", "", "附件 FileId (必填,来自 attachment list 的 FFileId)")
	cmd.Flags().StringVar(&out, "out", "", "输出路径 (目录或文件; 默认 ./<服务端文件名>)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "覆盖已存在的目标文件")
	return cmd
}
