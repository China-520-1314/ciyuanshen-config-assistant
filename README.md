# 词元神一键配置助手

这是一个 Wails v2 + Go + React/TypeScript 桌面工具，面向 Windows 用户自动检测本机 AI 客户端，并把词元神网关写入对应配置文件。用户可以登录词元神账号后选择分组并创建受限 API Key，也可以使用已有 API Key；选择工具和模型后，确认备份即可写入最新配置。

## 支持的客户端

- Claude Code
- Claude Desktop
- Codex
- Gemini CLI
- Grok Build
- OpenCode
- OpenClaw
- Hermes Agent

配置地址按客户端协议固定为：

- GPT、Grok、Codex 及 OpenAI 兼容工具：`https://api.ciyuanshen.top/v1`
- Claude：`https://api.ciyuanshen.top`
- Gemini CLI：`GOOGLE_GEMINI_BASE_URL=https://api.ciyuanshen.top`，并设置 `GOOGLE_GENAI_API_VERSION=v1`

Gemini 的 Base URL 不能直接写成 `https://api.ciyuanshen.top/v1`，因为 Gemini CLI 会自行拼接 `/v1beta` 路径。

## 配置方式

- 词元神账号模式：支持账号密码与两步验证登录，读取该账号已有的可用 API Key，并按目标工具检测模型后优先推荐；确认后可直接配置。没有合适的已有 Key 时，助手会创建名为“自动配置创建”的 Key，检测成功后自动完成配置。
- 已有 API Key 模式：输入现有 Key 后，助手通过 `/v1/models` 检测该 Key 可用且与目标客户端兼容的模型，再允许完成配置。
- 已配置的工具可以直接重新选择默认模型；助手会读取本机已有 Key 进行验证，无需再次把 Key 输入界面。

## 安全和恢复

- 用户输入的 API Key、账号会话和账号模式新建的原始 Key 只在当前进程内存中使用，不会保存到助手自己的数据库或浏览器存储。新建 Key 不会经过前端桥接，配置成功后才写入用户选定的客户端配置文件。
- 仓库公开的是接口调用逻辑，不包含用户令牌；发布前不得提交 `.env`、本机 `config.toml`、`auth.json`、日志或构建产物中的敏感内容。服务端仍必须做好鉴权、限流和审计。
- 写入目标客户端配置前会弹出确认，并在用户配置目录创建备份。
- 配置文件使用临时文件 + 原子替换；写入后会再次解析校验，失败时自动回滚。
- 备份页面显示备份目录，支持查看包含的文件、恢复和删除历史备份。
- 配置预览只显示文件路径和变更类型，不显示 API Key；配置查看器默认遮蔽敏感字段，仅在本机用户主动选择后才显示原始值。
- 配置后可对单个或已选工具执行“检测”，同时检查词元神配置字段和 `/v1/models` 网关连接。

## 分组倍率

“分组倍率”页从 `https://api.ciyuanshen.top/api/user/groups` 读取当前公开可见分组及实时基础倍率，并显示月卡 85 折、周卡 9 折后的参考倍率。该页面不读取或修改 NewAPI 数据库。

## Codex 固定模板

选择 Codex 时会先备份 `~/.codex/config.toml` 和 `~/.codex/auth.json`。如果已有 `config.toml`，助手会保留用户原来的 provider 名称（可以是 `custom`、`ciyuanshen` 或其他名称），让 `model_provider`、`[model_providers.<名称>]` 和表内 `name` 三处保持一致；遇到旧版本留下的重复 provider 表会在能确认属于当前 provider 时合并，并清理重复字段，同时保留真正无关的 provider 表。对于 `ciyuanshen` provider，`base_url` 会统一修正为 `https://api.ciyuanshen.top/v1`；已有模型、推理强度、注释和其他表段保持不变，缺失字段才按模板补齐。新文件默认使用 `gpt-5.6-terra`、`model_reasoning_effort = "max"`、实时网络搜索和 `https://api.ciyuanshen.top/v1` Responses 服务商，认证文件写入选定 API Key。

## 更新检查

应用会优先通过 GitHub Releases API 检查最新版本，并打开对应 Windows 安装包；无需额外部署下载站。

如果 GitHub 更新服务暂时不可用，应用会回退读取以下 HTTPS 更新清单：

`https://api.ciyuanshen.top/downloads/ciyuanshen-config-assistant/update.json`

清单格式见 [`update-manifest.example.json`](update-manifest.example.json)。`downloadUrl` 必须是 HTTPS 地址；应用只负责检查版本并打开下载地址，不会静默替换用户的可执行文件。

## 外观皮肤

“外观皮肤”页提供词元神青、动漫人物、樱花、雪山和夜景城市背景，图片随前端安装包内置，不依赖网络。用户也可以选择本地 JPG、PNG 或 WebP 图片，在 16:9 裁剪窗口中调整缩放和焦点后应用；裁剪结果会压缩后保存在本机 WebView 存储中，不会上传到服务器。内置壁纸来源和许可证见 [`frontend/src/assets/themes/SOURCES.md`](frontend/src/assets/themes/SOURCES.md)。

## 客户端安装与更新

助手会单独检查已支持客户端的本机版本与公开发布版本。可通过 npm 安装的 CLI 工具可在应用内执行固定的 npm 安装或更新命令；不支持自动安装的客户端会跳转到官方安装页。打开助手本身不会自动安装、更新客户端或发起多余的版本请求。

## 本地开发

环境要求：Go 1.22、Node.js 18+ 和 npm。

```bash
cd /opt/apps/ciyuanshen-config-assistant
go test ./...
cd frontend
npm ci
npm run build
cd ..
```

浏览器预览仅用于查看界面，不会读取本机配置、安装状态或 API Key；工具连接检测会明确提示需运行桌面安装版。工具与应用版本检查会读取公开发布源并显示实际最新版本：

```bash
cd frontend
npm run dev -- --host 0.0.0.0
```

桌面开发和生产构建需要 Wails CLI：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
```

## Windows `.exe` 打包

仓库中的 [`.github/workflows/windows.yml`](.github/workflows/windows.yml) 会在推送 `v*` 标签时使用 Windows runner 构建 amd64 NSIS 安装包，并上传到 GitHub Release。Release 只提供可安装的 `*-installer.exe` 和 `update.json`，避免用户误下载便携版。GitHub Release 是默认下载和更新来源；若需要自建下载站，可将同一安装包和更新清单部署到上述固定 URL。构建支持通过标签注入版本号，例如：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build \
  -platform windows/amd64 -nsis \
  -ldflags "-X main.appVersion=0.2.10"
```

## 设计边界

助手只修改用户明确选择的客户端配置，不安装或升级客户端本身，也不修改 NewAPI 服务端。若未来在服务器端部署本项目或更新静态资源，应按项目要求使用 4 个 CPU、低资源方式重建并重启。
