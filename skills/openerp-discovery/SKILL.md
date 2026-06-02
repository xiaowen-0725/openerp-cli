---
name: openerp-discovery
version: 0.1.0
description: "openerp 发现层：用 objects 搜索任意业务对象 FormId、用 schema 发现其字段、再用 query 查数据。当要查询的对象没有现成领域命令、或不确定 FormId/字段名时使用。"
metadata:
  requires:
    bins: ["openerp"]
  cliHelp: "openerp objects --help"
---

# openerp-discovery — 发现任意业务对象的查询方式

> 先读 [`../openerp-shared/SKILL.md`](../openerp-shared/SKILL.md)（鉴权、退出码、输出契约）。

金蝶业务对象上千个。**发现层**让你在运行期查到任意对象，无需等领域命令封装。高频对象已有领域命令（见 [`../openerp-domains/SKILL.md`](../openerp-domains/SKILL.md)），其余一律走这套三步发现。

## 三步发现工作流
```bash
# 1) 找对象 FormId（搜 BOS_ObjectType）
openerp objects --keyword 销售订单
#   → 行含 [FID, FName, FSubSystemId]，FID 即 FormId（如 SAL_SaleOrder）

# 2) 查该对象的可用字段
openerp schema SAL_SaleOrder --fields-only
#   → 各 entry 的字段 key/中文名/类型/关联对象(lookUpForm)

# 3) 用发现到的字段查数据
openerp query --form-id SAL_SaleOrder \
  --fields "FBillNo,FDate,FCustId.FName,FMaterialId.FNumber,FQty" \
  --filter "FBillNo='XSDD000006'" --top 20
```

## 关键规则（实测坐实，务必遵守）
- **字段 key 不要带 entry/分录前缀。** ExecuteBillQuery 把表头与分录字段拉平在同一命名空间：用分录字段就直接写它的 key（如 `FMaterialId`、`FQty`），**不要**写 `FEntity.FMaterialId` / `FSaleOrderEntry.FQty`（会报“标识为 FEntity 的字段不存在”）。
- **关联字段用点号下钻**：`FSupplierId.FName`、`FMaterialId.FNumber`、`FCustId.FNumber`。`schema` 输出里字段的 `lookUpForm` 非空即表示它是关联字段，可点号取关联对象的字段。
- **字段名大小写敏感**，以 `schema` 实测为准（同一概念在不同单据可能是 `FMaterialId` / `FMATERIALID` / `FMaterialID`）。
- **查询要带过滤或 --top**，避免全表扫描；超过 2000 行用 `--page-all`。
- 某字段整列为 `null`：多为无数据权限或字段名拼错 → 回到 `schema` 核对。

## 命名陷阱与不支持项
- **即时库存现量用 `STK_Inventory`（现存量余额对象），不是 `STK_InventoryQuery`**（后者是查询界面，无字段模型）。已封装为 `openerp inventory balance list`。
- **报表/账表**（模型类型 900、GUID 命名，如“销售订单执行明细表/销售明细表”）：**仍不支持**，需 `GetSysReportData`；或改用对应单据（如 `SAL_OUTSTOCK` 出库明细）复现。
- 个别对象 `QueryBusinessInfo` 返回空模型（如车间成本计算 `CB_SFCCostCalBill`）→ 无可查字段，不能走通用查询。

## objects 命令
```bash
openerp objects --keyword <名称关键词>     # 必须给收敛条件
openerp objects --subsystem <子系统>       # 按子系统过滤
openerp objects --keyword 委外 --top 30
```
## schema 命令
```bash
openerp schema <FormId>               # 完整元数据
openerp schema <FormId> --fields-only # 精简字段清单（推荐）
```
