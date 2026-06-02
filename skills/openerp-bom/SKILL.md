---
name: openerp-bom
version: 0.1.0
description: "金蝶云·星空 BOM(物料清单) 查询：按编号查看单个 BOM、按父物料列出子项,并给出采购价目(PUR_PriceCategory)查询配方。当用户要查 BOM 结构/子项/物料成本输入时使用。"
metadata:
  requires:
    bins: ["openerp"]
  cliHelp: "openerp bom --help"
---

# openerp-bom — BOM 查询

> **CRITICAL**：开始前先用 Read 读取 [`../openerp-shared/SKILL.md`](../openerp-shared/SKILL.md)（鉴权、退出码、输出契约都在那里）。

## 何时用
- 查某个 BOM 的子项（用料、用量分子/分母、损耗率、自制/外购标识）。
- 查 BOM 单据明细。
- 作为成本测算的输入（用料 × 价目）。

## 命令
| 命令 | 读/写 | 关键参数 |
|---|---|---|
| `openerp bom view --number "<BOM编号>"` | 读 | `--number`（必填），如 `"1.30.67.0132 VA0"` |
| `openerp bom list --material "<父物料编码>"` | 读 | `--material`（必填）、`--top N` |

`bom list` 返回的子项字段：子件编码/名称/规格、`FNumerator`(分子)、`FDenominator`(分母)、`FScrapRate`(损耗率%)、`FErpClsID`(1=外购/2=自制)、单位、BOM 编号。

## 示例
```bash
openerp bom view --number "1.30.67.0132 VA0"
openerp bom list --material "1.30.67.0132" --top 100
openerp bom list --material "1.30.67.0132" --format table   # 人类可读表格
```

## 兜底：用通用 query 查任意表单
领域命令没覆盖到时，回退到 `query`：
```bash
# 采购价目（验证锚点：物料 4.50.20.1549 价≈9.318584 / 含税10.53 / 税率13%）
openerp query --form-id PUR_PriceCategory \
  --fields "FMaterialId.FNumber,FPrice,FTaxPrice,FTaxRate,FDocumentStatus" \
  --filter "FMaterialId.FNumber='4.50.20.1549'" --jq '.data'
```

## 路由提示
- 物料**价格/采购价**查询 → `query --form-id PUR_PriceCategory`（非 BOM 命令）。
- 鉴权/网络/参数报错 → 先看退出码与 `hint`，跑 `openerp doctor`（见 `openerp-shared`）。
