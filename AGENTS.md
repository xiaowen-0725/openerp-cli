# AGENTS.md — openerp 贡献者 / AI 代理契约

> 本 CLI 的首要消费者是 **AI agent**。你写的每一条错误信息都会被 AI 解析以决定下一步动作。
> （理念取自 larksuite/cli。）让错误**结构化、可执行、带 hint**。

## 构建与测试
```bash
make build        # 编译到 bin/openerp
make test         # vet + gofmt 检查 + 单测
make unit-test    # 仅单测
```
离线构建：`GOPROXY=off`（依赖仅 spf13/cobra，已在 module cache）。

## 硬规则
- **结构化错误**：命令的 `RunE` 只返回 `errs.*` 类型错误（`errs.NewValidation/NewAuth/NewConfig/NewNetwork/NewAPI`），**禁止裸 `fmt.Errorf`**。根命令 `Execute()` 经 `output.EmitError` 把它渲染成 JSON 错误信封并映射退出码。
- **stdout=数据，stderr=其它**：成功信封写 stdout；`--verbose` 日志、错误信封写 stderr。命令通过 `f.IOStreams.{Out,Err}` 读写，绝不直接用 `os.Stdout`（便于测试注入）。
- **绝不打印机密**：`appSecret`、完整 `KDSVCSessionId` 一律掩码（`config.Mask` / `k3client.maskSession`）。
- **session 文件是 bearer token**：写 0600，按 profile 分文件，`.gitignore` 已忽略。
- **本轮只读**：没有写命令。未来写操作要走 `errs.ConfirmationRequiredError` + `--yes/--dry-run/--read-only` 护栏。

## 退出码（按错误 Category 路由，见 internal/output/exitcode.go）
| 码 | 含义 |
|----|------|
| 0  | 成功 |
| 1  | K3 业务错误 (api) |
| 2  | 参数校验 (validation) |
| 3  | 鉴权 / 配置失败 (authentication / configuration) |
| 4  | 网络错误 (network) |
| 5  | 内部错误 (internal) |
| 10 | 预留：写操作需 --yes (confirmation) |

## 源码分层
| 路径 | 职责 |
|------|------|
| `cmd/root.go` | 装配根命令、全局 flag、纯分组守卫、退出码路由 |
| `cmd/{config,auth,doctor,objects,schema,query,bom}/` | 各命令；领域命令由 catalog 数据驱动(`cmdutil/domaincmd.go` ← `internal/catalog/domains.json`，10 域/39 对象) |
| `internal/cmdutil/factory.go` | Factory(依赖注入) + IOStreams + `Config()`/`Client()` |
| `internal/cmdutil/run.go` | `RunBillQuery`/`RunView`/`RunBusinessInfo`：dry-run、分页、聚合(`--sum`/`--group-by`)、信封渲染的共享路径 |
| `internal/config/` | profile 凭据存储(0600)、env 覆盖、`Resolve` |
| `internal/k3client/` | **K3 鉴权内核**：LoginBySign 签名、`china_to_unicode`、session 复用、失效重登、ExecuteBillQuery/View |
| `internal/output/` | Envelope、退出码、json/ndjson/table/csv 渲染、jq 子集、错误信封 |
| `errs/` | 强类型错误契约（Problem + 各 Category 类型） |

## 测试要求
- 行为变更必须带测试。签名/编码这类纯函数有固定向量单测（`sha256SortedSign`、`chinaToUnicode`、dry-run wire golden）。
- 用 `t.Setenv("OPENERP_CONFIG_DIR", t.TempDir())` 隔离配置。
- 网络逻辑用 `httptest`（见 `k3client/client_test.go` 的失效重登测试）；**真实实例联调不进 CI**（无凭据）。

## 分发 / SKILL 约定
- 用户经 npm `@openydt/openerp-cli` 安装（`npm/` 壳包按平台下载 GitHub Release 二进制 + `npx skills` 同步技能）；打 `v*` tag 触发 `.github/workflows/release.yml`（goreleaser + npm publish）。详见 `RELEASING.md`。
- **对象经验沉淀 / 未配置引导是 SKILL.md 行为约定，非 Go 代码**：见 `skills/openerp-shared/SKILL.md` §6（对象经验）/§0（未配置引导）。CLI 侧只负责输出结构化错误 + 可执行 hint（如 `type=configuration`），由 agent 据此引导，零 Go 改动。

## 提交
- Conventional Commits：`feat: / fix: / docs: / test: / refactor: / chore:`。
- **绝不提交机密**（凭据放 `openerp config set`，落 `~/.config/openerp/`，不入库）。
