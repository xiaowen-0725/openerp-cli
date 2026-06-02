---
name: openerp-domains
version: 0.1.0
description: "openerp 领域查询命令总览：基础资料/采购/销售/库存(含即时库存)/生产/委外/计划/工程/车间/成本 共 10 域 39 个对象的人性化只读查询命令(list/view)。查这些高频业务对象时优先用本技能。"
metadata:
  requires:
    bins: ["openerp"]
  cliHelp: "openerp --help"
---

# openerp-domains — 领域查询命令（高频对象）

> 先读 [`../openerp-shared/SKILL.md`](../openerp-shared/SKILL.md)。没覆盖的对象走 [`../openerp-discovery/SKILL.md`](../openerp-discovery/SKILL.md)。

每个对象统一两个子命令：
- `openerp <domain> <object> list [过滤] [--top N] [--page-all] [--format json|table|csv]` —— **交易单据为单据级（每单一行，不展开分录）**
- `openerp <domain> <object> view --number <编号>` —— **看单条完整明细（含分录行）**

所有对象均已对真实实例联调通过。要看某单的物料/数量等分录明细，用 `view`。
**少数对象 list 本身就是明细/余额级（每行一个物料）**：`purchase price`(价目)、`inventory balance`(即时库存)、`production ppbom` / `subcontract ppbom`(用料清单)、`production mo`(生产订单产品)、`plan planorder`(计划)、`engineering route`(工艺路线)。
`list` 不带过滤即取前若干行；`--jq` 可裁剪；超过 ~2000 行用 `--page-all`。

## 命令清单（domain object → FormId）

| 命令 | 对象 | FormId | 常用过滤 |
|---|---|---|---|
| `base material` | 物料 | BD_MATERIAL | `--number` `--keyword` |
| `base customer` | 客户 | BD_Customer | `--number` `--keyword` |
| `base supplier` | 供应商 | BD_Supplier | `--number` `--keyword` |
| `base department` | 部门 | BD_Department | `--number` `--keyword` |
| `base stock` | 仓库 | BD_STOCK | `--number` `--keyword` |
| `base employee` | 员工 | BD_Empinfo | `--number` `--keyword` |
| `purchase requisition` | 采购申请 | PUR_Requisition | `--bill-no` `--date-from/-to` |
| `purchase order` | 采购订单 | PUR_PurchaseOrder | `--bill-no` `--supplier` `--date-from/-to` |
| `purchase instock` | 采购入库 | STK_InStock | `--bill-no` `--supplier` `--date-from/-to` |
| `purchase return` | 采购退料 | PUR_MRB | `--bill-no` `--date-from/-to` |
| `purchase price` | 采购价目 | PUR_PriceCategory | `--number` `--material` |
| `sales order` | 销售订单 | SAL_SaleOrder | `--bill-no` `--customer` `--date-from/-to` |
| `sales outstock` | 销售出库 | SAL_OUTSTOCK | `--bill-no` `--date-from/-to` |
| `sales delivery` | 发货通知 | SAL_DELIVERYNOTICE | `--bill-no` `--date-from/-to` |
| `sales return` | 销售退货 | SAL_RETURNSTOCK | `--bill-no` `--date-from/-to` |
| `sales quote` | 销售报价 | SAL_QUOTATION | `--bill-no` `--date-from/-to` |
| `sales returnnotice` | 退货通知 | SAL_RETURNNOTICE | `--bill-no` `--date-from/-to` |
| `inventory misc-in` | 其他入库 | STK_MISCELLANEOUS | `--bill-no` `--date-from/-to` |
| `inventory misc-out` | 其他出库 | STK_MisDelivery | `--bill-no` `--date-from/-to` |
| `inventory transfer` | 直接调拨 | STK_TransferDirect | `--bill-no` `--date-from/-to` |
| `inventory transfer-apply` | 调拨申请 | STK_TransferApply | `--bill-no` `--date-from/-to` |
| `inventory balance` | 即时库存现量 | STK_Inventory | `--material` `--stock` |
| `production mo` | 生产订单 | PRD_MO | `--bill-no` |
| `production ppbom` | 生产用料清单 | PRD_PPBOM | `--bill-no` `--mo` |
| `production pick` | 生产领料 | PRD_PICKMTRL | `--bill-no` `--date-from/-to` |
| `production report` | 生产汇报 | PRD_MORPT | `--bill-no` `--date-from/-to` |
| `production instock` | 生产入库 | PRD_INSTOCK | `--bill-no` `--date-from/-to` |
| `production return` | 生产退料 | PRD_ReturnMtrl | `--bill-no` `--date-from/-to` |
| `subcontract order` | 委外订单 | SUB_SUBREQORDER | `--bill-no` `--date-from/-to` |
| `subcontract ppbom` | 委外用料清单 | SUB_PPBOM | `--bill-no` |
| `subcontract pick` | 委外领料 | SUB_PICKMTRL | `--bill-no` `--date-from/-to` |
| `subcontract return` | 委外退料 | SUB_ReturnMtrl | `--bill-no` `--date-from/-to` |
| `plan planorder` | 计划订单 | PLN_PLANORDER | `--bill-no` `--material` |
| `plan forecast` | 预测单 | PLN_FORECAST | `--bill-no` `--date-from/-to` |
| `engineering route` | 工艺路线 | ENG_Route | `--number` `--material` |
| `engineering workcenter` | 工作中心 | ENG_WorkCenter | `--number` `--keyword` |
| `workshop process` | 工序 | PLM_STD_PROCESS | `--keyword` |
| `workshop in-output` | 车间投入产出 | CB_SFCINOUTPUTBILL | `--bill-no` |
| `costing dimension` | 核算维度 | BD_FLEXITEMPROPERTY | `--number` `--keyword` |

> BOM（物料清单 ENG_BOM）有独立命令：`openerp bom view --number` / `openerp bom list --material`（见 `../openerp-bom`）。

## 示例
```bash
openerp base material list --keyword 控制器 --format table
openerp purchase order list --supplier 01.001 --date-from 2024-01-01 --top 50
openerp sales order view --number XSDD000006
openerp inventory transfer list --date-from 2024-01-01 --page-all
openerp inventory balance list --material 2.60.51.0439          # 某物料各仓现存量
openerp purchase price list --material 4.50.20.1549 --jq '.data'
```

## 统计：--sum / --group-by（list 与 query 通用）
客户端聚合（自动把字段补进查询、自动取全量再算）：
```bash
# 5月销售总额(价税合计本位币) + 单数
openerp sales order list --date-from 2026-05-01 --date-to 2026-05-31 --sum FBillAllAmount_LC
#   → {"count":717,"sum":8672253.86,"sumField":"FBillAllAmount_LC"}

# 5月按客户分组(金额降序)
openerp sales order list --date-from 2026-05-01 --date-to 2026-05-31 \
  --sum FBillAllAmount_LC --group-by FCustId.FName
#   → [{"group":"润材电子商务有限公司","count":23,"sum":815733.38}, ...]

# 只分组计数(不求和)：每个供应商的采购订单数
openerp purchase order list --date-from 2026-01-01 --group-by FSupplierId.FName
```
要点：`--sum`/`--group-by` 的字段需是该对象可查字段（不在默认列里也会自动补查）；金额字段如 `FBillAllAmount`(价税合计)、`FBillAllAmount_LC`(本位币)、`FBillAmount`(不含税)、`FBillTaxAmount`(税额)。

## 想查清单外的对象？
用发现层：`objects --keyword <词>` 找 FormId → `schema <FormId>` 查字段 → `query --form-id ... --fields ...`（见 `../openerp-discovery`）。
