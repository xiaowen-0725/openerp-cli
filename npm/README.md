# @openydt/openerp-cli

金蝶云·星空 (Kingdee K3 Cloud) ERP CLI —— 为人和 AI Agent 而生。只读查询：BOM/物料/采购/销售/库存/生产等 + 对象发现层 + 对象经验沉淀。

```bash
npm i -g @openydt/openerp-cli
openerp --help
```

本包是壳：安装时（postinstall）按平台从 [GitHub Releases](https://github.com/xiaowen-0725/openerp-cli/releases) 下载对应的原生 Go 二进制，并 best-effort 把技能同步到本机各 AI agent（经 `npx skills`）。

源码、文档与 Agent 契约见仓库：<https://github.com/xiaowen-0725/openerp-cli>。
