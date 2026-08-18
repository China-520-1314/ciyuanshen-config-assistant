package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type UpdateInfo struct {
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion"`
	UpdateAvailable bool      `json:"updateAvailable"`
	DownloadURL     string    `json:"downloadUrl,omitempty"`
	ReleaseNotes    string    `json:"releaseNotes,omitempty"`
	PublishedAt     string    `json:"publishedAt,omitempty"`
	SHA256          string    `json:"sha256,omitempty"`
	CheckedAt       time.Time `json:"checkedAt"`
	Error           string    `json:"error,omitempty"`
}

type updateManifest struct {
	Version      string `json:"version"`
	DownloadURL  string `json:"downloadUrl"`
	ReleaseNotes string `json:"releaseNotes"`
	PublishedAt  string `json:"publishedAt"`
	SHA256       string `json:"sha256"`
}

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	PublishedAt string               `json:"published_at"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func checkGitHubRelease(client *http.Client, current, releaseURL string) UpdateInfo {
	result := UpdateInfo{CurrentVersion: current, CheckedAt: time.Now()}
	request, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		result.Error = "GitHub 更新地址无效"
		return result
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "CiyuanShen-Config-Assistant/"+current)
	response, err := client.Do(request)
	if err != nil {
		result.Error = "暂时无法连接 GitHub 更新服务"
		return result
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		result.Error = "GitHub 尚未发布可用版本"
		return result
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result.Error = fmt.Sprintf("GitHub 更新服务返回 HTTP %d", response.StatusCode)
		return result
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 512*1024)).Decode(&release); err != nil {
		result.Error = "GitHub Release 数据格式无效"
		return result
	}
	version := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if version == "" {
		result.Error = "GitHub Release 缺少版本标签"
		return result
	}
	result.LatestVersion = version
	result.ReleaseNotes = strings.TrimSpace(release.Body)
	if result.ReleaseNotes == "" {
		result.ReleaseNotes = strings.TrimSpace(release.Name)
	}
	result.PublishedAt = release.PublishedAt
	result.UpdateAvailable = compareVersions(version, current) > 0

	asset := selectWindowsInstaller(release.Assets)
	if asset.BrowserDownloadURL != "" {
		if !strings.HasPrefix(strings.ToLower(asset.BrowserDownloadURL), "https://") {
			result.Error = "GitHub 安装包地址不是 HTTPS"
			return result
		}
		result.DownloadURL = asset.BrowserDownloadURL
	}
	if result.UpdateAvailable && result.DownloadURL == "" {
		result.Error = "GitHub Release 未包含 Windows 安装包"
	}
	return result
}

func selectWindowsInstaller(assets []githubReleaseAsset) githubReleaseAsset {
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".exe") && strings.Contains(name, "installer") {
			return asset
		}
	}
	return githubReleaseAsset{}
}

func checkForUpdates(client *http.Client, current, manifestURL string) UpdateInfo {
	result := UpdateInfo{CurrentVersion: current, CheckedAt: time.Now()}
	request, err := http.NewRequest(http.MethodGet, manifestURL, nil)
	if err != nil {
		result.Error = "更新地址无效"
		return result
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "CiyuanShen-Config-Assistant/"+current)
	response, err := client.Do(request)
	if err != nil {
		result.Error = "暂时无法连接更新服务"
		return result
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result.Error = fmt.Sprintf("更新服务返回 HTTP %d", response.StatusCode)
		return result
	}
	limited := io.LimitReader(response.Body, 512*1024)
	var manifest updateManifest
	if err := json.NewDecoder(limited).Decode(&manifest); err != nil {
		result.Error = "更新清单格式无效"
		return result
	}
	manifest.Version = strings.TrimSpace(manifest.Version)
	if manifest.Version == "" {
		result.Error = "更新清单缺少版本号"
		return result
	}
	if manifest.DownloadURL != "" && !strings.HasPrefix(strings.ToLower(manifest.DownloadURL), "https://") {
		result.Error = "更新下载地址不是 HTTPS"
		return result
	}
	result.LatestVersion = manifest.Version
	result.DownloadURL = manifest.DownloadURL
	result.ReleaseNotes = manifest.ReleaseNotes
	result.PublishedAt = manifest.PublishedAt
	result.SHA256 = manifest.SHA256
	result.UpdateAvailable = compareVersions(manifest.Version, current) > 0
	return result
}

func compareVersions(left, right string) int {
	left = strings.TrimPrefix(strings.TrimSpace(left), "v")
	right = strings.TrimPrefix(strings.TrimSpace(right), "v")
	leftMain, leftPre := splitVersion(left)
	rightMain, rightPre := splitVersion(right)
	for index := 0; index < 3; index++ {
		leftNumber := versionPart(leftMain, index)
		rightNumber := versionPart(rightMain, index)
		if leftNumber != rightNumber {
			if leftNumber > rightNumber {
				return 1
			}
			return -1
		}
	}
	if leftPre == rightPre {
		return 0
	}
	if leftPre == "" {
		return 1
	}
	if rightPre == "" {
		return -1
	}
	return strings.Compare(leftPre, rightPre)
}

func splitVersion(version string) (string, string) {
	parts := strings.SplitN(version, "-", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func versionPart(version string, index int) int {
	parts := strings.Split(version, ".")
	if index >= len(parts) {
		return 0
	}
	value := 0
	for _, character := range parts[index] {
		if character < '0' || character > '9' {
			return value
		}
		value = value*10 + int(character-'0')
	}
	return value
}
