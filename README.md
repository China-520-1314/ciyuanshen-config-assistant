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

- GPT、Grok、Codex 及 OpenAI 兼容工具：`https://ciyuanshen.top/v1`
- Claude：`https://ciyuanshen.top`
- Gemini CLI：`GOOGLE_GEMINI_BASE_URL=https://ciyuanshen.top`，并设置 `GOOGLE_GENAI_API_VERSION=v1`

Gemini 的 Base URL 不能直接写成 `https://ciyuanshen.top/v1`，因为 Gemini CLI 会自行拼接 `/v1beta` 路径。

## 配置方式

- 词元神账号模式：支持账号密码与两步验证登录，读取该账号可用的分组和模型，为选定工具创建仅限对应分组与模型的新 API Key；创建后会先检测可用模型，再写入本机客户端配置。
- 已有 API Key 模式：输入现有 Key 后，助手通过 `/v1/models` 检测该 Key 可用且与目标客户端兼容的模型，再允许完成配置。
- 已配置的工具可以直接重新选择默认模型；助手会读取本机已有 Key 进行验证，无需再次把 Key 输入界面。

## 安全和恢复

- 用户输入的 API Key、账号会话和账号模式新建的原始 Key 只在当前进程内存中使用，不会保存到助手自己的数据库或浏览器存储。新建 Key 不会经过前端桥接，配置成功后才写入用户选定的客户端配置文件。
- 写入目标客户端配置前会弹出确认，并在用户配置目录创建备份。
- 配置文件使用临时文件 + 原子替换；写入后会再次解析校验，失败时自动回滚。
- 备份页面显示备份目录，支持查看包含的文件、恢复和删除历史备份。
- 配置预览只显示文件路径和变更类型，不显示 API Key；配置查看器默认遮蔽敏感字段，仅在本机用户主动选择后才显示原始值。
- 配置后可对单个或已选工具执行“检测”，同时检查词元神配置字段和 `/v1/models` 网关连接。

## 分组倍率

“分组倍率”页从 `https://ciyuanshen.top/api/user/groups` 读取当前公开可见分组及实时基础倍率，并显示月卡 85 折、周卡 9 折后的参考倍率。该页面不读取或修改 NewAPI 数据库。

## Codex 固定模板

选择 Codex 时会覆盖 `~/.codex/config.toml` 和 `~/.codex/auth.json`。配置文件使用 `ciyuanshen` Responses 服务商，写入用户选定的默认 `model`，并固定使用 `review_model = "gpt-5.6-sol"`、`model_reasoning_effort = "medium"`、`service_tier = "fast"` 和实时网络搜索；认证文件写入选定 API Key。原文件会先进入备份目录。

## 更新检查

应用会优先通过 GitHub Releases API 检查最新版本，并打开对应 Windows 安装包；无需额外部署下载站。

如果 GitHub 更新服务暂时不可用，应用会回退读取以下 HTTPS 更新清单：

`https://ciyuanshen.top/downloads/ciyuanshen-config-assistant/update.json`

清单格式见 [`update-manifest.example.json`](update-manifest.example.json)。`downloadUrl` 必须是 HTTPS 地址；应用只负责检查版本并打开下载地址，不会静默替换用户的可执行文件。

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
  -ldflags "-X main.appVersion=0.2.2"
```

## 设计边界

助手只修改用户明确选择的客户端配置，不安装或升级客户端本身，也不修改 NewAPI 服务端。若未来在服务器端部署本项目或更新静态资源，应按项目要求使用 4 个 CPU、低资源方式重建并重启。
