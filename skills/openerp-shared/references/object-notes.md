# 对象经验（object-notes）

按业务对象（FormId）积累的经验存在 **openerp 配置目录**下的 `object-notes/{FormId}.{profile}.md`（默认 `~/.config/openerp/object-notes/`，或 `$OPENERP_CONFIG_DIR/object-notes/`；父目录即 `openerp config path` 所示文件的所在目录）。例如 `PUR_PriceCategory.prod.md`。**一对象一 profile 一文件，物理隔离不同实例/数据中心**——不同 profile 是不同的 K3 实例，字段定制、价格、数据权限都可能不同，避免把某实例经验误用到另一实例。**不要**放进技能目录（`skills/`）——技能会被同步覆盖，经验会丢。

对标 `github.com/eze-is/web-access` 的「站点经验」与自家 `openydt-cli` 的 `park-notes`：经验是**给 agent 读的先验知识**，不是会自动套进真实请求的配置。理念是「记录已验证的技术事实，当提示而非保证」。

## 任务开始前（回忆）
1. 先确认当前 profile：`openerp config list`（`*` 标当前）或 `--profile` 显式指定。
2. `ls ~/.config/openerp/object-notes/`（目录不存在或为空属正常）。文件名形如 `{FormId}.{profile}.md`；需按业务名（如「销售订单」）匹配时，读各文件 frontmatter 的 `aliases`。
3. 确定目标对象（FormId 或别名）后，**若有匹配该 FormId 且 profile 与当前一致的文件，先 Read 它**：据此选已验证字段、复用有效查询、避开已知陷阱、参照验证锚点。
4. 经验标注 `updated` 日期，**当「可能有效的提示」而非保证**；按经验操作若失败 → 回退通用三步发现流程（见 `../../openerp-discovery/SKILL.md`），并**更新**该文件对应条目。

## 查询/发现成功后（沉淀）
5. 命令成功返回（`ok:true`，退出码 0）后，若发现该对象值得记录的**已验证**新事实——可用的字段 key、点号关联路径、有效过滤写法、某对象无字段模型/不支持、稳定的验证锚点数据——主动**追加/更新**到 `object-notes/{FormId}.{profile}.md`（当前 profile）对应小节并刷新 frontmatter `updated`。文件不存在则按下方模板创建（先 `mkdir -p` 该目录）。只记录在**当前 profile** 验证过的事实。
6. **只写经过验证的事实，不写猜测。** 宁可漏记，不可错记。
7. **隐私 / 数据红线**：`验证锚点` 小节只记**少量代表性记录**用于佐证查询可用，**绝不批量转存业务数据**；**不记录可识别个人信息**（客户/供应商联系人、手机号等 PII）。frontmatter 的 `profile` 须与文件名一致。
8. 清理：`rm ~/.config/openerp/object-notes/{FormId}.{profile}.md`。

## 文件格式（文件名 `{FormId}.{profile}.md`）
frontmatter 机读 + 正文 LLM 读：
- `formId`（主键，与文件名前缀一致）
- `aliases`（业务俗称数组，便于「销售订单」这类自然语言匹配到 FormId）
- `title`（一句话说明）
- `profile`（经验来自哪个 profile/实例，与文件名 profile 一致）
- `updated`（发现/更新日期 YYYY-MM-DD）

正文四节：**对象特征 / 有效字段与查询 / 已知陷阱 / 验证锚点或已知数据**。

## 种子示例 1：`PUR_PriceCategory.prod.md`（含验证锚点）
```markdown
---
formId: PUR_PriceCategory
aliases: [采购价目, 采购价目表]
title: 采购价目（明细级，每物料一行）
profile: prod
updated: 2026-06-02
---
## 对象特征
- 明细级对象：list 返回每物料一行（非单据头）；numberField=FNumber，支持 view。
## 有效字段与查询
- 已验证字段：FMaterialId.FNumber, FPrice(未税), FTaxPrice(含税), FTaxRate, FDocumentStatus。
- 典型查询：
  openerp query --form-id PUR_PriceCategory \
    --fields "FMaterialId.FNumber,FPrice,FTaxPrice,FTaxRate" \
    --filter "FMaterialId.FNumber='4.50.20.1549'" --jq .data
## 已知陷阱
- 关联字段必须点号下钻（FMaterialId.FNumber）；整列 null 多为字段名拼错或无数据权限 → 回 schema 核对。
## 验证锚点 / 已知数据
- 物料 4.50.20.1549：FPrice≈9.318584(未税) / FTaxPrice=10.53(含税) / FTaxRate=13%。
  与 Python 原型 VERIFY_ANCHOR 及 CLAUDE.md 一致（prod profile，2026-06-02 实测）。
```

## 种子示例 2：`SAL_SaleOrder.prod.md`（aliases 匹配 + 字段前缀陷阱 + 负面经验）
```markdown
---
formId: SAL_SaleOrder
aliases: [销售订单]
title: 销售订单
profile: prod
updated: 2026-06-02
---
## 对象特征
- 单据头 + 分录；ExecuteBillQuery 把表头与分录字段拉平在同一命名空间。
## 有效字段与查询
- 已验证字段：FBillNo, FDate, FCustId.FName, FMaterialId.FNumber, FQty。
- 典型查询：
  openerp query --form-id SAL_SaleOrder \
    --fields "FBillNo,FDate,FCustId.FName,FMaterialId.FNumber,FQty" \
    --filter "FBillNo='XSDD000006'" --top 20
## 已知陷阱
- **字段 key 不带分录前缀**：用 FMaterialId / FQty，不要写 FEntity.FMaterialId（会报「标识为 FEntity 的字段不存在」）。
- **报表/账表（模型类型 900，如销售明细表）不支持**通用查询，需 GetSysReportData；改用对应单据（如 SAL_OUTSTOCK 出库明细）复现。
```
