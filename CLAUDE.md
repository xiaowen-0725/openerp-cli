# CLAUDE.md — openerp 开发指南

金蝶云·星空 ERP CLI（Go + Cobra）。本仓库当前是**只读可行性 POC**。

## 构建/测试
```bash
make build          # -> bin/openerp
make test           # vet + gofmt 检查 + go test ./...
GOPROXY=off go build ./...   # 离线可构建（依赖仅 spf13/cobra，已缓存）
```

## 架构速览
- 入口 `main.go` → `cmd.Execute()`：建 Factory、跑 cobra、用 `output.EmitError` 把 `errs.*` 错误渲染为 JSON 信封 + 退出码。
- **K3 鉴权内核在 `internal/k3client/`**（这是与 openydt 无状态签名最大的不同）：
  - `LoginBySign`：`sign = sha256(sorted([acctId,userName,appId,appSecret,ts]))`；`parameters=[acctId,userName,appId,ts,sign,lcid]`；成功取 `KDSVCSessionId`。
  - `china_to_unicode`：把 parameters 串里 0x4E00–0x29FA5 的 CJK 转成 `\uXXXX`（K3 怪癖，移植自 Python）。
  - 请求带 `kdservice-sessionid` 头；响应原文含「登录」→ 清 session + 重登 + 重试一次。
  - `ExecuteBillQuery` / `View`：`parameters` 是「含一个 JSON 字符串元素的数组」。
- 输出契约在 `internal/output/`，错误类型在 `errs/`（见 `AGENTS.md`）。

## 约定
- 命令只通过 `f.IOStreams` 读写；只返回 `errs.*` 错误（禁裸 `fmt.Errorf`）。
- 机密一律掩码；session 文件 0600 且 `.gitignore`。
- 纯函数（签名/编码）有固定向量单测；网络逻辑用 `httptest`。

## 移植来源 / 参考
- 鉴权与查询逻辑：`/Users/zhoujw/develop/tmp/ai_jieti/k3cloud_bom_cost_explorer.py`（已调通的 Python 原型）。
- 房屋风格：`/Users/zhoujw/develop/tmp/openydt-cli/`。
- Agent-first 契约（envelope/errs/doctor/api 命令/IOStreams）：`/Users/zhoujw/develop/github/cli/`（larksuite/cli）。

## 验证锚点（联调）
物料 `4.50.20.1549` 采购价 ≈ 9.318584（未税）/ 10.53（含税）/ 13% 税率 —— 与 Python `VERIFY_ANCHOR` 一致。
此锚点也是「对象经验」层的种子示例：写在 `~/.config/openerp/object-notes/PUR_PriceCategory.{profile}.md` 的 `验证锚点` 小节（规范见 `skills/openerp-shared/references/object-notes.md`）。
