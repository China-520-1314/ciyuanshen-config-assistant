# 慈元神一键配置助手

这是一个 Wails v2 + Go + React/TypeScript 桌面工具，面向 Windows 用户自动检测本机 AI 客户端，并把慈元神网关写入对应配置文件。用户只需输入自己的 API Key、选择工具和模型，即可预览、备份并一键配置。

## 支持的客户端

- Claude Code
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

## 安全和恢复

- API Key 只在当前进程内存中使用，不会保存到助手自己的数据库或浏览器存储。
- 写入目标客户端配置前，会在用户配置目录创建备份。
- 配置文件使用临时文件 + 原子替换；写入后会再次解析校验，失败时自动回滚。
- 备份页面可以恢复最近 20 次配置。
- 预览页面只显示文件路径和变更类型，不显示 API Key。

## 更新检查

应用启动后可以在“版本更新”页面通过 HTTPS 请求更新清单：

`https://ciyuanshen.top/downloads/ciyuanshen-config-assistant/update.json`

清单格式见 [`update-manifest.example.json`](update-manifest.example.json)。`downloadUrl` 必须是 HTTPS 地址；应用只负责检查版本并打开下载地址，不会静默替换用户的可执行文件。

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

浏览器预览（不调用 Go 方法，会使用 mock 数据）：

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

仓库中的 [`.github/workflows/windows.yml`](.github/workflows/windows.yml) 会在推送 `v*` 标签时使用 Windows runner 构建 amd64 NSIS 安装包，并上传到 GitHub Release。发布时应把安装包放到更新清单的 `downloadUrl` 对应地址，再将清单部署到上面的固定 URL。构建支持通过标签注入版本号，例如：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build \
  -platform windows/amd64 -nsis \
  -ldflags "-X main.appVersion=0.1.1"
```

## 设计边界

助手只修改用户明确选择的客户端配置，不安装或升级客户端本身，也不修改 NewAPI 服务端。若未来在服务器端部署本项目或更新静态资源，应按项目要求使用 4 个 CPU、低资源方式重建并重启。

