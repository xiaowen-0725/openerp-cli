// postinstall: 按平台从 GitHub Releases 下载对应的 openerp 二进制(版本与本包一致)。
const fs = require("fs");
const path = require("path");
const os = require("os");
const { execFileSync } = require("child_process");

const pkg = require("./package.json");
const REPO = "xiaowen-0725/openerp-cli";

const plat = { darwin: "darwin", linux: "linux", win32: "windows" }[process.platform];
const arch = { x64: "amd64", arm64: "arm64" }[process.arch];
if (!plat || !arch) {
  console.error(`[openerp] 不支持的平台: ${process.platform}/${process.arch}`);
  process.exit(1);
}

const ext = plat === "windows" ? "zip" : "tar.gz";
const asset = `openerp-cli_${pkg.version}_${plat}_${arch}.${ext}`;
const url = `https://github.com/${REPO}/releases/download/v${pkg.version}/${asset}`;
const binDir = path.join(__dirname, "bin");
fs.mkdirSync(binDir, { recursive: true });
const archive = path.join(binDir, asset);

(async () => {
  console.log("[openerp] 下载", url);
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok) throw new Error(`下载失败 HTTP ${res.status} — 确认已发布 v${pkg.version}`);
  fs.writeFileSync(archive, Buffer.from(await res.arrayBuffer()));

  if (ext === "zip") execFileSync("unzip", ["-o", archive, "-d", binDir], { stdio: "inherit" });
  else execFileSync("tar", ["-xzf", archive, "-C", binDir], { stdio: "inherit" });

  const src = path.join(binDir, plat === "windows" ? "openerp.exe" : "openerp");
  const dst = path.join(binDir, plat === "windows" ? "openerp-bin.exe" : "openerp-bin");
  fs.renameSync(src, dst);
  fs.chmodSync(dst, 0o755);
  fs.unlinkSync(archive);
  console.log("[openerp] 安装完成");

  // best-effort: 同步 AI Agent 技能到本机已装的各 agent(失败绝不影响安装)
  syncSkills();
})().catch((e) => {
  console.error("[openerp]", e.message);
  process.exit(1);
});

// 把技能同步到本机所有已装 agent;失败只 warn,postinstall 不报错退出。
function syncSkills() {
  try {
    execFileSync("npx", ["-y", "skills", "add", REPO, "-g", "-y"], { stdio: "inherit" });
    writeSkillsState();
    console.log("[openerp] skills 已同步");
  } catch (e) {
    console.warn(`[openerp] skills 同步失败(可稍后手动:npx skills add ${REPO} -g):${e.message}`);
  }
}

// 写 skills-state.json 作基准(落 openerp 配置目录,与 config.Dir() 默认一致)。
function writeSkillsState() {
  const base = process.env.OPENERP_CONFIG_DIR
    ? process.env.OPENERP_CONFIG_DIR
    : path.join(process.env.XDG_CONFIG_HOME || path.join(os.homedir(), ".config"), "openerp");
  fs.mkdirSync(base, { recursive: true });
  const state = {
    version: pkg.version,
    last_attempt_version: pkg.version,
    updated_at: new Date().toISOString(),
  };
  fs.writeFileSync(path.join(base, "skills-state.json"), JSON.stringify(state, null, 2) + "\n");
}
