# 发布流程(npm)

面向使用者的安装方式是 **npm**(`npm i -g @openydt/openerp-cli`)。npm 包是个壳,安装时按平台从
**GitHub Releases** 下载原生二进制——所以一次发布 = 先出 GitHub Release(二进制),再发 npm 包。
两步都由 **打 tag** 自动完成(GitHub Actions,见 `.github/workflows/release.yml`)。

## 一次性准备
在 GitHub 仓库 `Settings → Secrets and variables → Actions` 添加:
- **`NPM_TOKEN`**:npm 账号的发布 token(npmjs.com → Access Tokens → Generate,类型选 Automation/Publish)。
  没有它,GitHub Release 仍会发布,但 npm 不会自动发(CI 会给 warning 并跳过)。

## 每次发布
```bash
go test ./... && make build         # 确保 main 上构建/测试通过
git tag v0.1.0
git push origin v0.1.0              # 打标签即触发发布
```
`release.yml` 会:
1. `go test` → `goreleaser release`:在 GitHub Releases 发布 darwin/linux/windows × amd64/arm64 归档 + checksums。
2. 把 `npm/package.json` 版本同步为 `0.1.0`,`npm publish --access public` 发布到公共 npm(需 NPM_TOKEN)。

用户随后:`npm i -g @openydt/openerp-cli`(postinstall 自动下载对应平台二进制 + 同步技能)。

## 手动发 npm(不走 CI,或 CI 未配 NPM_TOKEN 时)
GitHub Release v0.1.0 已存在的前提下,本地直接发对应版本的 npm 壳包:
```bash
npm login                                   # 需 npm 账号
cd npm
npm version 0.1.0 --no-git-tag-version --allow-same-version   # 与 GitHub Release 的 v0.1.0 对齐
npm publish --access public
```

## 备注
- 包名作用域 `@openydt`(npm 组织,zhoujw0725 为 owner):scoped 包发布必须带 `--access public`(package.json 已含 `publishConfig.access=public`)。
- 二进制版本号由 tag 注入(`-ldflags -X .../cmd/version.Version`);本地 `make build` 默认是 `0.1.0-poc`。
- npm 包 `version` 必须与已发布的 GitHub Release tag(`v<version>`)一致,否则 postinstall 下载 404。
- **自动更新**:发布版二进制每天静默自检 GitHub Releases `latest`,有新版则后台校验 SHA256(`checksums.txt`)后原子替换自身,下次启动生效。dev 构建(`0.1.0-poc` 等带后缀的版本)不会自更新。因此**每次发版务必让 goreleaser 产出 `checksums.txt`**(`.goreleaser.yml` 的 `checksum:` 已配置)且 Release 资产名形如 `openerp-cli_<ver>_<os>_<arch>.tar.gz`——自更新按此规则拼 URL。用户可 `OPENERP_NO_UPDATE=1` 关闭。
  - 注意:自更新只换二进制,**不重新同步技能**(技能随 `npm i` 的 postinstall 同步)。技能有重大变更时需提示用户 `npm i -g @openydt/openerp-cli@latest`。
