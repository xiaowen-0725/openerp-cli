# CLAUDE.md — openerp 开发指南

金蝶云·星空 ERP CLI（Go + Cobra）。本仓库当前是**只读可行性 POC**。

## 构建/测试
```bash
make build          # -> bin/openerp
make test           # vet + gofmt 检查 + go test ./...
GOPROXY=off go build ./...   # 离线可构建（依赖仅 spf13/cobra，已缓存）
```

## 分发 / 发布（详见 `RELEASING.md`）
- 用户经 npm 安装 **`@openydt/openerp-cli`**（`npm/` 壳包）：postinstall 按平台从 GitHub Releases 下原生二进制 + best-effort `npx skills add xiaowen-0725/openerp-cli` 同步技能。
- **打 `v*` tag** → `.github/workflows/release.yml`：`goreleaser`(`.goreleaser.yml`) 构建 darwin/linux/windows × amd64/arm64 归档发 GitHub Release，再（有 `NPM_TOKEN` secret 时）`npm publish --access public`。
- 版本经 `-ldflags -X github.com/xiaowen-0725/openerp-cli/cmd/version.Version` 注入（源在 leaf 包 `cmd/version`，避免自更新逻辑 import 循环）；本地 `make build` 默认 `0.1.0-poc`。npm 包 `version` 必须 == 已发布的 `v<version>` Release，否则 postinstall 下载 404。
- **自动更新在 `internal/selfupdate/`**：root 的 `PersistentPreRun` 每天静默自检 GitHub Releases `latest`，有新版则 fork 脱离会话的子进程(`openerp __selfupdate-apply <ver>`)下载→校验 `checksums.txt` SHA256→原子替换当前二进制，下次启动生效（stdout 的 JSON 契约绝不被触碰）。`openerp update [--check]` 是显式入口。dev 构建(`-poc` 等带后缀)与 `OPENERP_NO_UPDATE=1`/`--no-update-check` 时关闭；无写权限(全局安装)时静默放弃，显式命令回退提示 `npm i`。

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
- **对象经验沉淀 / 未配置引导是 SKILL.md 约定（零 Go 代码）**：按 FormId 在 `~/.config/openerp/object-notes/{FormId}.{profile}.md` 沉淀已验证事实（查询前回忆、查通后写回）、无 profile 时引导配置——均写在 `skills/openerp-shared/SKILL.md`（§6/§0）+ `references/object-notes.md`。改行为改技能文件，不动 Go。

## 移植来源 / 参考
- 鉴权与查询逻辑：`/Users/zhoujw/develop/tmp/ai_jieti/k3cloud_bom_cost_explorer.py`（已调通的 Python 原型）。
- 房屋风格：`/Users/zhoujw/develop/tmp/openydt-cli/`。
- Agent-first 契约（envelope/errs/doctor/api 命令/IOStreams）：`/Users/zhoujw/develop/github/cli/`（larksuite/cli）。

## 验证锚点（联调）
物料 `4.50.20.1549` 采购价 ≈ 9.318584（未税）/ 10.53（含税）/ 13% 税率 —— 与 Python `VERIFY_ANCHOR` 一致。
此锚点也是「对象经验」层的种子示例：写在 `~/.config/openerp/object-notes/PUR_PriceCategory.{profile}.md` 的 `验证锚点` 小节（规范见 `skills/openerp-shared/references/object-notes.md`）。
