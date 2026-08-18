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
