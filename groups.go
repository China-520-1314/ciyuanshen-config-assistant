package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const publicGroupRatiosURL = "https://api.ciyuanshen.top/api/user/groups"

// GroupRatio is a publicly visible CiyuanShen routing group and its current
// base billing multiplier.
type GroupRatio struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Ratio       float64 `json:"ratio"`
}

type GroupRatioReport struct {
	Groups    []GroupRatio `json:"groups"`
	Endpoint  string       `json:"endpoint"`
	FetchedAt time.Time    `json:"fetchedAt"`
}

type groupRatioAPIResponse struct {
	Success bool                        `json:"success"`
	Message string                      `json:"message"`
	Data    map[string]groupRatioRecord `json:"data"`
}

type groupRatioRecord struct {
	Description string          `json:"desc"`
	Ratio       json.RawMessage `json:"ratio"`
}

func (a *App) GetPublicGroupRatios() (GroupRatioReport, error) {
	return fetchPublicGroupRatios(a.client, publicGroupRatiosURL)
}

func fetchPublicGroupRatios(client *http.Client, endpoint string) (GroupRatioReport, error) {
	if client == nil {
		client = http.DefaultClient
	}
	report := GroupRatioReport{Endpoint: endpoint, FetchedAt: time.Now()}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return report, errors.New("无法创建分组倍率请求")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ciyuanshen-config-assistant")

	response, err := client.Do(request)
	if err != nil {
		return report, fmt.Errorf("无法读取词元神分组倍率：%w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return report, errors.New("读取分组倍率响应失败")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return report, fmt.Errorf("读取分组倍率失败：HTTP %d", response.StatusCode)
	}

	var payload groupRatioAPIResponse
	if err := json.Unmarshal(content, &payload); err != nil {
		return report, errors.New("分组倍率响应格式无法识别")
	}
	if !payload.Success {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = "服务端未返回分组倍率"
		}
		return report, errors.New(message)
	}

	for name, item := range payload.Data {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var ratio float64
		if err := json.Unmarshal(item.Ratio, &ratio); err != nil || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			// "auto" and other non-numeric routing entries are not a billable group.
			continue
		}
		report.Groups = append(report.Groups, GroupRatio{
			Name:        name,
			Description: strings.TrimSpace(item.Description),
			Ratio:       ratio,
		})
	}
	if len(report.Groups) == 0 {
		return report, errors.New("当前没有可展示的公共分组倍率")
	}
	sort.Slice(report.Groups, func(left, right int) bool {
		if report.Groups[left].Ratio == report.Groups[right].Ratio {
			return report.Groups[left].Name < report.Groups[right].Name
		}
		return report.Groups[left].Ratio < report.Groups[right].Ratio
	})
	return report, nil
}
