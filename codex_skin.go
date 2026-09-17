package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Renderer and selector contract from Codex-Dream-Skin (MIT).
// Pinned at 34335d27d54300eccb325cc652f6c93fef428b84; see skin_runtime/LICENSE.
//
//go:embed skin_runtime/*
var skinAssets embed.FS

const skinPort = "19329"
const maxSkinFile = 15 << 20

type CodexSkin struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Background string `json:"background"`
	Accent     string `json:"accent"`
	Appearance string `json:"appearance"`
	Opacity    int    `json:"opacity"`
	Blur       int    `json:"blur"`
	FontSize   int    `json:"fontSize"`
	FocusX     int    `json:"focusX"`
	FocusY     int    `json:"focusY"`
	Favorite   bool   `json:"favorite"`
}
type CodexSkinLibrary struct {
	Schema int         `json:"schema"`
	Themes []CodexSkin `json:"themes"`
}
type skinTarget struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
	WS    string `json:"webSocketDebuggerUrl"`
}

func skinLibraryPath() string { return filepath.Join(filepath.Dir(backupRoot()), "codex-skins.json") }
func readSkinFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxSkinFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxSkinFile {
		return nil, errors.New("皮肤文件超过 15MB")
	}
	return b, nil
}
func validateSkin(s CodexSkin) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`).MatchString(s.ID) || strings.TrimSpace(s.Name) == "" || len(s.Name) > 120 {
		return errors.New("皮肤名称或编号无效")
	}
	if !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(s.Accent) {
		return errors.New("主题色必须为六位十六进制颜色")
	}
	if s.Appearance != "auto" && s.Appearance != "light" && s.Appearance != "dark" {
		return errors.New("外观模式无效")
	}
	if s.Opacity < 15 || s.Opacity > 100 || s.Blur < 0 || s.Blur > 30 || s.FontSize < 12 || s.FontSize > 22 || s.FocusX < 0 || s.FocusX > 100 || s.FocusY < 0 || s.FocusY > 100 {
		return errors.New("皮肤参数超出范围")
	}
	_, err := skinImage(s.Background)
	return err
}
func skinImage(data string) ([]byte, error) {
	prefix, encoded, ok := strings.Cut(data, ",")
	if !ok || (prefix != "data:image/png;base64" && prefix != "data:image/jpeg;base64") {
		return nil, errors.New("仅支持 PNG/JPEG 本地背景图")
	}
	if len(encoded) > 14<<20 {
		return nil, errors.New("背景图不得超过 10MB")
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(b) > 10<<20 {
		return nil, errors.New("背景图编码无效或过大")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 24000000 {
		return nil, errors.New("图片尺寸或格式无效（最多 2400 万像素）")
	}
	if _, _, err = image.Decode(bytes.NewReader(b)); err != nil {
		return nil, errors.New("图片解码失败")
	}
	return b, nil
}
func builtInSkins() []CodexSkin {
	names := []string{"星海蓝", "森林晨光", "暮色紫", "暖沙金", "玫瑰夜色", "极简石墨"}
	accents := []string{"#5cbcff", "#73c69a", "#b69dff", "#e6b972", "#ed9bae", "#aebccc"}
	result := []CodexSkin{}
	for i, name := range names {
		img := image.NewRGBA(image.Rect(0, 0, 960, 540))
		for y := 0; y < 540; y++ {
			for x := 0; x < 960; x++ {
				v := uint8(18 + x*22/960 + y*18/540)
				img.SetRGBA(x, y, color.RGBA{v + uint8(i*3), v + uint8((5-i)*3), v + 22, 255})
			}
		}
		var b bytes.Buffer
		_ = png.Encode(&b, img)
		result = append(result, CodexSkin{ID: fmt.Sprintf("builtin-%d", i), Name: name, Background: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes()), Accent: accents[i], Appearance: "dark", Opacity: 75, Blur: 8, FontSize: 15, FocusX: 50, FocusY: 50})
	}
	return result
}
func (a *App) GetCodexSkins() (CodexSkinLibrary, error) {
	a.skinMu.Lock()
	defer a.skinMu.Unlock()
	b, err := readSkinFile(skinLibraryPath())
	if errors.Is(err, os.ErrNotExist) {
		return CodexSkinLibrary{Schema: 1, Themes: builtInSkins()}, nil
	}
	if err != nil {
		return CodexSkinLibrary{}, err
	}
	var lib CodexSkinLibrary
	if json.Unmarshal(b, &lib) != nil || lib.Schema != 1 {
		return lib, errors.New("本地皮肤库格式无效")
	}
	return lib, nil
}
func (a *App) SaveCodexSkin(s CodexSkin) error {
	if err := validateSkin(s); err != nil {
		return err
	}
	a.skinMu.Lock()
	defer a.skinMu.Unlock()
	lib := CodexSkinLibrary{Schema: 1, Themes: builtInSkins()}
	b, err := readSkinFile(skinLibraryPath())
	if err == nil {
		if json.Unmarshal(b, &lib) != nil {
			return errors.New("皮肤库损坏，请先备份")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	found := false
	for i := range lib.Themes {
		if lib.Themes[i].ID == s.ID {
			lib.Themes[i] = s
			found = true
			break
		}
	}
	if !found {
		lib.Themes = append(lib.Themes, s)
	}
	b, err = json.Marshal(lib)
	if err != nil {
		return err
	}
	if len(b) > maxSkinFile {
		return errors.New("皮肤库已满，请减少背景图片大小")
	}
	return atomicWrite(skinLibraryPath(), b)
}
func (a *App) DeleteCodexSkin(id string) error {
	a.skinMu.Lock()
	defer a.skinMu.Unlock()
	b, err := readSkinFile(skinLibraryPath())
	if err != nil {
		return err
	}
	var lib CodexSkinLibrary
	if json.Unmarshal(b, &lib) != nil {
		return errors.New("皮肤库格式错误")
	}
	themes := []CodexSkin{}
	for _, s := range lib.Themes {
		if s.ID != id {
			themes = append(themes, s)
		}
	}
	lib.Themes = themes
	b, _ = json.Marshal(lib)
	return atomicWrite(skinLibraryPath(), b)
}
func (a *App) SelectCodexSkinBackground() (string, error) {
	path, err := wr.OpenFileDialog(a.ctx, wr.OpenDialogOptions{Title: "选择 Codex 背景图", Filters: []wr.FileFilter{{DisplayName: "PNG / JPEG", Pattern: "*.png;*.jpg;*.jpeg"}}})
	if err != nil || path == "" {
		return "", err
	}
	b, err := readSkinFile(path)
	if err != nil {
		return "", err
	}
	mime := http.DetectContentType(b)
	data := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
	_, err = skinImage(data)
	return data, err
}
func (a *App) ImportCodexSkin() (*CodexSkin, error) {
	path, err := wr.OpenFileDialog(a.ctx, wr.OpenDialogOptions{Title: "导入词元神皮肤 JSON", Filters: []wr.FileFilter{{DisplayName: "词元神皮肤", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return nil, err
	}
	b, err := readSkinFile(path)
	if err != nil {
		return nil, err
	}
	var s CodexSkin
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&s) != nil {
		return nil, errors.New("仅支持词元神皮肤 JSON；DreamSkin ZIP 请通过原项目客户端导入")
	}
	s.ID = fmt.Sprintf("import-%d", time.Now().UnixNano())
	if err = a.SaveCodexSkin(s); err != nil {
		return nil, err
	}
	return &s, nil
}
func (a *App) ExportCodexSkin(s CodexSkin) error {
	if err := validateSkin(s); err != nil {
		return err
	}
	path, err := wr.SaveFileDialog(a.ctx, wr.SaveDialogOptions{Title: "导出皮肤", DefaultFilename: "codex-skin.json", Filters: []wr.FileFilter{{DisplayName: "皮肤 JSON", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return atomicWrite(path, b)
}
func skinTargets() ([]skinTarget, error) {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get("http://127.0.0.1:" + skinPort + "/json/list")
	if err != nil {
		return nil, errors.New("尚未连接 Codex：请完全退出 Codex，再点击“启动 Codex 换肤模式”")
	}
	defer resp.Body.Close()
	var targets []skinTarget
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&targets); err != nil {
		return nil, errors.New("调试接口返回无效数据")
	}
	result := []skinTarget{}
	for _, t := range targets {
		u, e := url.Parse(t.URL)
		if e != nil || t.Type != "page" {
			continue
		}
		if u.Scheme == "app" && u.Host == "codex" && !strings.Contains(t.URL, "avatar-overlay") {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("未找到 Codex 主窗口，请打开 Codex 对话主界面")
	}
	return result, nil
}
func skinEvaluate(target skinTarget, expression string) (json.RawMessage, error) {
	u, err := url.Parse(target.WS)
	if err != nil || u.Scheme != "ws" || u.Host != "127.0.0.1:"+skinPort || !strings.HasPrefix(u.Path, "/devtools/page/") {
		return nil, errors.New("拒绝非本机 Codex 调试地址")
	}
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	c, _, err := dialer.Dial(target.WS, nil)
	if err != nil {
		return nil, errors.New("连接 Codex 调试窗口失败")
	}
	defer c.Close()
	c.SetReadLimit(2 << 20)
	_ = c.SetReadDeadline(time.Now().Add(15 * time.Second))
	_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err = c.WriteJSON(map[string]any{"id": 1, "method": "Runtime.evaluate", "params": map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}}); err != nil {
		return nil, err
	}
	for {
		var r struct {
			ID     int             `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Result struct {
					Value json.RawMessage `json:"value"`
				} `json:"result"`
				Exception json.RawMessage `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err = c.ReadJSON(&r); err != nil {
			return nil, errors.New("Codex 渲染确认超时")
		}
		if r.ID != 1 {
			continue
		}
		if len(r.Error) > 0 || len(r.Result.Exception) > 0 {
			return nil, errors.New("Codex 未能应用皮肤，可能需要更新适配")
		}
		return r.Result.Result.Value, nil
	}
}
func skinPayload(s CodexSkin) string {
	css, _ := skinAssets.ReadFile("skin_runtime/base.css")
	renderer, _ := skinAssets.ReadFile("skin_runtime/renderer.js")
	selectors, _ := skinAssets.ReadFile("skin_runtime/selectors.json")
	css = append(css, []byte(fmt.Sprintf("\n:root[data-dream-skin=active] {font-size:%dpx!important} :root[data-dream-skin=active] [data-ds-part=sidebar], :root[data-dream-skin=active] [data-ds-part=composer] {opacity:1;backdrop-filter:blur(%dpx);background-color:color-mix(in srgb,var(--ds-panel) %d%%,transparent)!important}", s.FontSize, s.Blur, s.Opacity))...)
	theme := map[string]any{"id": s.ID, "name": s.Name, "appearance": s.Appearance, "colors": map[string]string{"accent": s.Accent}, "colorMode": "explicit", "explicitColorKeys": []string{"accent"}, "art": map[string]any{"focusX": float64(s.FocusX) / 100, "focusY": float64(s.FocusY) / 100, "taskMode": "ambient", "safeArea": "left"}}
	sum := sha256.Sum256([]byte(s.Background))
	theme["artKey"] = hex.EncodeToString(sum[:])
	encode := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	replacements := []string{"__DREAM_SKIN_SELECTORS_JSON__", string(selectors), "__DREAM_SKIN_CSS_JSON__", encode(string(css)), "__DREAM_SKIN_ART_JSON__", encode(s.Background), "__DREAM_SKIN_THEME_JSON__", encode(theme), "__DREAM_SKIN_VERSION_JSON__", encode("ciyuanshen-" + appVersion), "__DREAM_SKIN_STYLE_REVISION_JSON__", encode(s.ID), "__DREAM_SKIN_PAYLOAD_REVISION_JSON__", encode(s.ID)}
	return strings.NewReplacer(replacements...).Replace(string(renderer))
}
func (a *App) ApplyCodexSkin(s CodexSkin) (string, error) {
	if err := validateSkin(s); err != nil {
		return "", err
	}
	a.skinMu.Lock()
	defer a.skinMu.Unlock()
	targets, err := skinTargets()
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		expression := "(async()=>{const result=" + skinPayload(s) + ";await new Promise(r=>setTimeout(r,500));return !!result?.installed && document.documentElement.getAttribute('data-dream-skin')==='active' && !!window.__CODEX_DREAM_SKIN_STATE__;})()"
		v, e := skinEvaluate(t, expression)
		if e != nil {
			return "", e
		}
		if string(v) != "true" {
			return "", errors.New("未确认皮肤生效，请点击恢复默认后重试")
		}
	}
	return "皮肤已应用到 Codex。Codex 完全退出后需重新启动换肤模式并应用。", nil
}
func (a *App) RestoreCodexSkin() (string, error) {
	a.skinMu.Lock()
	defer a.skinMu.Unlock()
	targets, err := skinTargets()
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		v, e := skinEvaluate(t, "(()=>{window.__CODEX_DREAM_SKIN_STATE__?.cleanup?.();return !window.__CODEX_DREAM_SKIN_STATE__ && !document.documentElement.hasAttribute('data-dream-skin');})()")
		if e != nil {
			return "", e
		}
		if string(v) != "true" {
			return "", errors.New("恢复未确认，请退出 Codex 后正常打开")
		}
	}
	return "已移除皮肤，恢复 Codex 原始界面。", nil
}
func (a *App) LaunchCodexSkinMode() (string, error) {
	profile := filepath.Join(filepath.Dir(backupRoot()), "codex-skin-profile")
	if err := os.MkdirAll(profile, 0700); err != nil {
		return "", err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "Codex", "--args", "--remote-debugging-address=127.0.0.1", "--remote-debugging-port="+skinPort, "--user-data-dir="+profile)
	case "windows":
		script := `$ErrorActionPreference='Stop'; $p=Get-AppxPackage -Name 'OpenAI.Codex' | Sort-Object Version -Descending | Select-Object -First 1; if(!$p){throw 'Codex not installed'}; $exe=Join-Path $p.InstallLocation 'app\Codex.exe'; if(!(Test-Path -LiteralPath $exe)){throw 'Codex executable not found'}; Start-Process -FilePath $exe -ArgumentList '--remote-debugging-address=127.0.0.1','--remote-debugging-port=` + skinPort + `',` + powershellQuote(`"--user-data-dir=`+profile+`"`)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	default:
		return "", errors.New("换肤启动支持 Windows 和 macOS")
	}
	if err := cmd.Run(); err != nil {
		return "", errors.New("启动失败，请确认已安装官方 Codex 桌面端，并完全退出后再试")
	}
	return "已请求启动 Codex。等待主窗口打开后点击应用皮肤；如果连接失败，请完全退出旧 Codex 再启动。", nil
}
