# 词元神一键配置助手

这是一个 Wails v2 + Go + React/TypeScript 桌面工具，面向 Windows 和 macOS 用户自动检测本机 AI 客户端，并把词元神网关写入对应配置文件。用户可以登录词元神账号后选择分组并创建受限 API Key，也可以使用已有 API Key；选择工具和模型后，确认备份即可写入最新配置。

## 支持的客户端

- Claude Code CLI/插件
- Claude Desktop
- ChatGPT/Codex CLI/Codex插件
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
- 一键配置在写入并校验成功后会尝试自动重启可安全定位的桌面应用。CLI 和编辑器插件不会被强制关闭，以免中断当前会话；无法安全自动重启时，界面会提示用户手动关闭并重新打开对应工具。

## 安全和恢复

### 模型路由：在 Codex 里使用 Claude、Gemini、Grok、DeepSeek

侧栏打开“模型路由”，按三个步骤操作：

1. 选择“使用 Codex 当前的词元神 Key”，或粘贴另一个词元神 Key，点击“读取可用模型”。
2. 搜索并选择实际模型。列表来自该 Key 的 `/v1/models` 权限，模型需要支持 Chat Completions；完成代码操作还需要支持工具调用。
3. 点击“一键启动并配置 Codex”，然后重新打开 Codex、新建对话。使用期间保持助手运行。更换模型时先停止，再选择并启动。

Codex 显示 `gpt-5.6-terra` 兼容别名，助手页面同时显示实际目标模型；本地路由发送给词元神的 `model` 始终为所选目标，不会自行回退到其他模型。网关内部的渠道映射仍由网关决定。模型列表验证不代表上游一定可用，实际请求失败会显示 HTTP 状态或协议错误。

本地服务只监听 `127.0.0.1` 的随机端口，使用独立随机令牌认证，不允许浏览器跨域调用。它将 Codex Responses 请求转换为词元神 `/v1/chat/completions`，支持流式/非流式文本、图片输入、函数工具、命名空间工具、自由文本工具（包括补丁）以及工具结果回传。目标模型需具备相应能力。当前不支持内置联网搜索、`previous_response_id` 服务端续接、远端压缩等扩展；长对话请新建会话，不支持的输入会明确报错。

启动时仅临时切换 `~/.codex/config.toml`，保留原文件的完整字节备份，不修改 `auth.json`。上游 Key 仅在内存中使用，写入 Codex 配置的是本机临时令牌。临时配置会关闭联网搜索、请求压缩和部分不兼容的上下文扩展，并取消默认 profile 选择；不要通过命令行 `--profile`/`-c` 再覆盖路由配置。自定义 `CODEX_HOME` 暂不支持一键接管。

点击“停止并恢复配置”或正常退出时恢复原文件；异常退出后重新打开助手，在路由页点击“停止并恢复配置”。如果原配置已被其他程序修改，助手不会覆盖修改，恢复记录仍保存在用户配置目录的 `CiyuanShen/Config Assistant/router-recovery.json`（macOS 位于 `~/Library/Application Support/`，Windows 位于 `%AppData%`）。该 JSON 的 `original` 字段是原文件的 Base64 内容，`installed` 是路由写入的内容。恢复记录可能包含原配置中的凭据，请勿公开分享。路由启用或有待处理恢复记录期间，助手会阻止其他配置写入和备份恢复，避免相互覆盖。

设计参考 [CC Switch](https://github.com/farion1231/cc-switch)（MIT，Jason Young）的本地代理、Responses/Chat 转换和工具兼容处理；本项目使用独立实现的 Go 路由，没有引入其 Rust 运行时。Codex 服务商字段按 [OpenAI 官方配置参考](https://learn.chatgpt.com/docs/config-file/config-reference) 核对。

开发验证：`go test -race ./...` 覆盖协议转换、鉴权、模型转发、流中断、配置恢复和冲突保护。安装了 Codex CLI 时，可用 `CIYUANSHEN_TEST_CODEX_CLI=1 go test -run TestRouterCodexCLIContract -v` 运行真实 CLI 协议测试；仅调用本地模拟上游并使用临时用户目录，不需要真实 Key。

### 通用配置保护

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

Codex 配置弹窗默认勾选三个可切换选项：

- `context_management = { experimental_mode = true }`
- `token_budget.enabled = true`
- `token_budget.use_history_notes_extension = true`

用户可在写入前取消任一选项。已有的内联配置、点号配置或 `[context_management]`、`[token_budget]` 表会就地补全和更新，不会产生重复 TOML 表。

## 更新检查

应用启动时会优先读取词元神自建更新源；检测到新版本会在窗口中央询问用户是否更新，并按当前系统选择对应安装包：Windows 选择 NSIS 安装包，macOS 选择 Universal DMG。Windows 支持下载后自动关闭旧进程并安装；macOS 会提供官方 DMG/ZIP 下载地址，首次安装仍需用户在 Finder 中确认打开。

默认更新清单为：

`https://api.ciyuanshen.top/downloads/ciyuanshen-config-assistant/update.json`

静态镜像同步失败或更新清单不可用时，应用会回退 GitHub Releases。用户确认更新后，客户端会优先下载并校验中国优化线路的安装包；连接、响应、读取、保存、大小或 SHA-256 校验失败时，会自动下载并校验同一版本的 GitHub Release 资产。清单格式见 [`update-manifest.example.json`](update-manifest.example.json)。`downloadUrl` 必须是 HTTPS 地址；Windows 只会在用户确认后交接给安装程序，不会在后台静默替换用户的可执行文件。

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

## Windows 与 macOS 打包

仓库中的 [`.github/workflows/windows.yml`](.github/workflows/windows.yml) 是统一的跨平台 Release 流程。推送 `v*` 标签后，它会并行构建 Windows amd64 NSIS 安装包和 macOS Universal 应用，并在两端构建成功后一次性上传 GitHub Release。Release 会包含：

- Windows：`*-installer.exe` 和 Windows 专用 `update.json`。
- macOS：`*-macos-universal.dmg`（推荐）和 `*-macos-universal.zip`（备用）。Universal 包同时支持 Intel 与 Apple Silicon Mac。

macOS 构建必须在 macOS runner 上执行，不能在 Linux/Windows 主机上交叉打包 Wails 的 Cocoa WebView。工作流使用 `macos-14` runner、Wails `darwin/universal` 目标和系统 `hdiutil` 制作 DMG；应用没有 Apple Developer 签名或公证时，用户首次打开需要在 Finder 中右键应用选择“打开”，或在“系统设置 → 隐私与安全性”中允许。

GitHub Release 用于构建产物归档和更新源故障回退。部署在更新服务器上的 `ciyuanshen-config-assistant-release-sync.timer` 会以低优先级镜像最新 Release 到 `api.ciyuanshen.top`，客户端优先从该静态源下载。Windows 本地构建示例：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build \
  -platform windows/amd64 -nsis \
  -ldflags "-X main.appVersion=0.2.19"
```

macOS 本地构建示例（需要 macOS、Xcode Command Line Tools 和 `hdiutil`）：

```bash
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build \
  -platform darwin/universal \
  -ldflags "-X main.appVersion=0.2.19"
```

Wails 会先生成 `build/bin/ciyuanshen-config-assistant.app`；发布流程再将它打成 DMG 和 ZIP。Release 标签、`wails.json` 的产品版本和应用内版本号必须保持一致。

## 设计边界

助手只修改用户明确选择的客户端配置，不安装或升级客户端本身，也不修改 NewAPI 服务端。若未来在服务器端部署本项目或更新静态资源，应按项目要求使用 4 个 CPU、低资源方式重建并重启。
