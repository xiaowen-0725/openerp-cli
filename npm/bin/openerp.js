#!/usr/bin/env node
// 透传 wrapper: 调用 postinstall 下载好的原生二进制。
const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const binName = process.platform === "win32" ? "openerp-bin.exe" : "openerp-bin";
const bin = path.join(__dirname, binName);
if (!fs.existsSync(bin)) {
  console.error("[openerp] 未找到二进制, 请重新安装: npm i -g @openydt/openerp-cli");
  process.exit(1);
}
const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(r.status == null ? 1 : r.status);
