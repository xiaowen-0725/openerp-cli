# openerp

金蝶云·星空 (Kingdee K3 Cloud) ERP CLI —— 为人和 AI Agent 而生。

> **可行性 POC（只读）**。打通「配置凭据 → LoginBySign 鉴权 → 真实只读查询 → 干净 JSON → agent 经 SKILL.md 消费」的端到端链路。
> 写操作、代码生成、多领域命令、skillsync 等为后续阶段（见 `cmd` 注释与计划文档）。

## 快速开始
```bash
make build

# 1) 配置一个 profile（凭据落 ~/.config/openerp/config.json, 0600）
openerp config set --profile prod \
  --server-url "https://akeparking.ik3cloud.com/K3Cloud/" \
  --acct-id <数据中心ID> --user <用户名> \
  --app-id <APP_ID> --app-secret <APP_SECRET> --lcid 2052

openerp config list          # appSecret 掩码显示
openerp doctor               # 自检：配置 → 登录 → 一次探测查询
openerp auth test            # 单独验证凭据（LoginBySign）

# 2) 通用查询（ExecuteBillQuery）
openerp query --form-id ENG_BOM \
  --fields "FMATERIALIDCHILD.FNumber,FMATERIALIDCHILD.FName,FNumerator" \
  --filter "FMATERIALID.FNumber='1.30.67.0132'" --top 100

# 3) 领域命令（演示 skill 包装的人性化 UX）
openerp bom view --number "1.30.67.0132 VA0"
openerp bom list --material "1.30.67.0132"

# 4) 价目表 + jq 取值
openerp query --form-id PUR_PriceCategory \
  --fields "FMaterialId.FNumber,FPrice,FTaxPrice,FTaxRate,FDocumentStatus" \
  --filter "FMaterialId.FNumber='4.50.20.1549'" --jq '.data'
```

## 全局开关
`--profile` `--format json|ndjson|table|csv` `--jq <路径子集>` `--dry-run` `--verbose` `--read-only`

`--dry-run` 打印将发送的请求（含外层 body、endpoint、掩码 session），不实际发送 —— 排查签名/编码必备。

## 鉴权模型（与众不同处）
金蝶星空是**有状态**鉴权：`LoginBySign` 登录一次拿到 `KDSVCSessionId`，后续请求带 `kdservice-sessionid` 头复用，直到失效再自动重登。session 缓存在 `~/.config/openerp/session-<profile>.json`（0600，**是 bearer token，勿外传**）。

## 给 AI Agent
- 输出契约见 [`AGENTS.md`](AGENTS.md)：结构化错误信封、退出码、stdout=数据/stderr=其它。
- Skills：[`skills/openerp-shared`](skills/openerp-shared/SKILL.md)（基座，先读）、[`skills/openerp-bom`](skills/openerp-bom/SKILL.md)（领域）。
- **对象经验沉淀**（对标 `eze-is/web-access` 站点经验）：按 K3 对象 FormId 把已验证的字段/过滤/陷阱/锚点存到 `~/.config/openerp/object-notes/{FormId}.{profile}.md`，agent 查询前回忆、查通后沉淀，跨 session 复用。纯文件约定（零 Go 代码）、只读不存 PII、按 profile 隔离。规范见 [`skills/openerp-shared/references/object-notes.md`](skills/openerp-shared/references/object-notes.md)。

## 开发
```bash
make test     # vet + gofmt + 单测（离线：GOPROXY=off）
```
依赖仅 `spf13/cobra`。参考基准：自家 `openydt-cli`（房屋风格）+ `larksuite/cli`（agent-first 契约）。
