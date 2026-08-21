package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	dashboardBaseURL       = "https://api.ciyuanshen.top"
	provisionLifetime      = 20 * time.Minute
	maximumAltchaAttempts  = 1000000
	dashboardTokenPageSize = 100
	dashboardTokenMaxPages = 1000
)

type AccountLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AccountTwoFactorRequest struct {
	FlowToken string `json:"flowToken"`
	Code      string `json:"code"`
}

type AccountState struct {
	SignedIn         bool      `json:"signedIn"`
	Username         string    `json:"username"`
	Balance          string    `json:"balance"`
	Quota            int64     `json:"quota"`
	BalanceUpdatedAt time.Time `json:"balanceUpdatedAt"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type AccountLoginResult struct {
	SignedIn          bool      `json:"signedIn"`
	RequiresTwoFactor bool      `json:"requiresTwoFactor"`
	FlowToken         string    `json:"flowToken"`
	Username          string    `json:"username"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ToolGroupOption struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Ratio       string  `json:"ratio"`
	Models      []Model `json:"models"`
}

type ToolOptionsResponse struct {
	ClientID     string            `json:"clientId"`
	Groups       []ToolGroupOption `json:"groups"`
	ExistingKeys []ToolKeyResult   `json:"existingKeys,omitempty"`
}

type ToolKeyRequest struct {
	ClientID string `json:"clientId"`
	Group    string `json:"group"`
}

// ToolKeyResult deliberately contains an opaque provision id instead of the
// generated key. The raw key remains in the Go process until it is written to
// the selected local client configuration.
type ToolKeyResult struct {
	ProvisionID string  `json:"provisionId"`
	ClientID    string  `json:"clientId"`
	Group       string  `json:"group"`
	Name        string  `json:"name,omitempty"`
	Existing    bool    `json:"existing,omitempty"`
	Models      []Model `json:"models"`
	Status      int     `json:"status"`
	Endpoint    string  `json:"endpoint"`
}

type ToolKeyValidationRequest struct {
	ClientID string `json:"clientId"`
	APIKey   string `json:"apiKey"`
}

type ToolKeyValidationResult struct {
	ClientID      string  `json:"clientId"`
	Models        []Model `json:"models"`
	SelectedModel string  `json:"selectedModel,omitempty"`
	Status        int     `json:"status"`
	Endpoint      string  `json:"endpoint"`
}

type ToolConfigurationRequest struct {
	ClientID string `json:"clientId"`
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
}

type ProvisionedToolConfigurationRequest struct {
	ProvisionID string `json:"provisionId"`
	ClientID    string `json:"clientId"`
	Model       string `json:"model"`
}

// ExistingToolConfigurationRequest applies a newly selected default model with
// the API Key that is already stored by the selected client. The key never
// crosses the frontend bridge for this flow.
type ExistingToolConfigurationRequest struct {
	ClientID string `json:"clientId"`
	Model    string `json:"model"`
}

type dashboardSession struct {
	AccessToken string
	Username    string
	Balance     string
	Quota       int64
	UpdatedAt   time.Time
	ExpiresAt   time.Time
}

type dashboardAccountStatus struct {
	QuotaPerUnit               float64 `json:"quota_per_unit"`
	QuotaDisplayType           string  `json:"quota_display_type"`
	USDExchangeRate            float64 `json:"usd_exchange_rate"`
	CustomCurrencySymbol       string  `json:"custom_currency_symbol"`
	CustomCurrencyExchangeRate float64 `json:"custom_currency_exchange_rate"`
}

type provisionedToolKey struct {
	ClientID string
	Group    string
	Name     string
	Key      string
	Models   []Model
	Existing bool
	Created  time.Time
}

// dashboardToken is the masked token record returned by the account API. The
// unmasked key is fetched separately and never crosses the Wails bridge.
type dashboardToken struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Status             int    `json:"status"`
	ExpiredTime        int64  `json:"expired_time"`
	RemainQuota        int64  `json:"remain_quota"`
	UnlimitedQuota     bool   `json:"unlimited_quota"`
	Group              string `json:"group"`
	CreatedTime        int64  `json:"created_time"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	ModelLimits        string `json:"model_limits"`
}

const automaticToolKeyName = "自动配置创建"

type dashboardEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type dashboardGroup struct {
	Description string          `json:"desc"`
	Ratio       json.RawMessage `json:"ratio"`
}

type altchaChallenge struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	MaxNumber int    `json:"maxnumber"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
}

type altchaPayload struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	Number    int    `json:"number"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
}

func (a *App) GetAccountState() AccountState {
	a.accountMu.RLock()
	session := a.account
	a.accountMu.RUnlock()
	if session.AccessToken == "" {
		return AccountState{}
	}
	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		a.clearAccountSession()
		return AccountState{}
	}
	return accountStateFromSession(session)
}

// RefreshAccountState updates the in-memory account summary. It deliberately
// does not persist the dashboard token, user data, or quota to local storage.
func (a *App) RefreshAccountState() (AccountState, error) {
	accessToken, err := a.accountAccessToken()
	if err != nil {
		return AccountState{}, err
	}

	data, _, err := a.dashboardData(http.MethodGet, "/api/user/self", accessToken, nil)
	if err != nil {
		return AccountState{}, err
	}
	var profile struct {
		Username string      `json:"username"`
		Quota    json.Number `json:"quota"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		return AccountState{}, errors.New("词元神账户信息格式无法识别")
	}
	quota, err := profile.Quota.Int64()
	if err != nil {
		return AccountState{}, errors.New("词元神账户余额格式无法识别")
	}

	status := dashboardAccountStatus{QuotaPerUnit: 500000, QuotaDisplayType: "USD"}
	statusData, _, statusErr := a.dashboardData(http.MethodGet, "/api/status", "", nil)
	if statusErr == nil {
		if err := json.Unmarshal(statusData, &status); err != nil {
			status = dashboardAccountStatus{QuotaPerUnit: 500000, QuotaDisplayType: "USD"}
		}
	}

	a.accountMu.Lock()
	session := a.account
	if session.AccessToken == "" || session.AccessToken != accessToken {
		a.accountMu.Unlock()
		return AccountState{}, errors.New("登录状态已过期，请重新登录")
	}
	if username := strings.TrimSpace(profile.Username); username != "" {
		session.Username = username
	}
	session.Quota = quota
	session.Balance = formatAccountBalance(quota, status)
	session.UpdatedAt = time.Now()
	a.account = session
	a.accountMu.Unlock()

	return accountStateFromSession(session), nil
}

func accountStateFromSession(session dashboardSession) AccountState {
	return AccountState{
		SignedIn:         true,
		Username:         session.Username,
		Balance:          session.Balance,
		Quota:            session.Quota,
		BalanceUpdatedAt: session.UpdatedAt,
		ExpiresAt:        session.ExpiresAt,
	}
}

func formatAccountBalance(quota int64, status dashboardAccountStatus) string {
	quotaPerUnit := status.QuotaPerUnit
	if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
		quotaPerUnit = 500000
	}
	amount := float64(quota) / quotaPerUnit
	switch strings.ToUpper(strings.TrimSpace(status.QuotaDisplayType)) {
	case "TOKENS":
		return formatAccountInteger(quota) + " 额度"
	case "CNY":
		rate := status.USDExchangeRate
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			rate = 1
		}
		return "¥" + formatAccountAmount(amount*rate)
	case "CUSTOM":
		rate := status.CustomCurrencyExchangeRate
		if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
			rate = 1
		}
		symbol := strings.TrimSpace(status.CustomCurrencySymbol)
		if symbol == "" {
			symbol = "¤"
		}
		return symbol + formatAccountAmount(amount*rate)
	default:
		return "$" + formatAccountAmount(amount)
	}
}

func formatAccountAmount(amount float64) string {
	precision := 2
	if amount != 0 && math.Abs(amount) < 1 {
		precision = 4
	}
	return strconv.FormatFloat(amount, 'f', precision, 64)
}

func formatAccountInteger(value int64) string {
	text := strconv.FormatInt(value, 10)
	start := 0
	if strings.HasPrefix(text, "-") {
		start = 1
	}
	for index := len(text) - 3; index > start; index -= 3 {
		text = text[:index] + "," + text[index:]
	}
	return text
}

func (a *App) LogoutAccount() {
	a.clearAccountSession()
}

func (a *App) LoginAccount(request AccountLoginRequest) (AccountLoginResult, error) {
	username := strings.TrimSpace(request.Username)
	if username == "" || strings.TrimSpace(request.Password) == "" {
		return AccountLoginResult{}, errors.New("请输入词元神账号和密码")
	}

	altcha, err := a.requestAltchaToken()
	if err != nil {
		return AccountLoginResult{}, fmt.Errorf("登录验证准备失败：%w", err)
	}
	_, raw, err := a.dashboardRaw(http.MethodPost, "/api/user/login", "", map[string]string{
		"username": username,
		"password": request.Password,
		"altcha":   altcha,
	})
	if err != nil {
		return AccountLoginResult{}, err
	}
	return a.consumeLoginResponse(raw, username)
}

func (a *App) VerifyAccountTwoFactor(request AccountTwoFactorRequest) (AccountLoginResult, error) {
	if strings.TrimSpace(request.FlowToken) == "" || strings.TrimSpace(request.Code) == "" {
		return AccountLoginResult{}, errors.New("请输入两步验证代码")
	}
	_, raw, err := a.dashboardRaw(http.MethodPost, "/api/user/login/2fa", "", map[string]string{
		"flow_token": strings.TrimSpace(request.FlowToken),
		"code":       strings.TrimSpace(request.Code),
	})
	if err != nil {
		return AccountLoginResult{}, err
	}
	return a.consumeLoginResponse(raw, "")
}

func (a *App) GetAccountToolOptions(clientID string) (ToolOptionsResponse, error) {
	clientID, err := normaliseClientID(clientID)
	if err != nil {
		return ToolOptionsResponse{}, err
	}
	return a.accountToolOptions(clientID, true)
}

func (a *App) accountToolOptions(clientID string, includeExisting bool) (ToolOptionsResponse, error) {
	accessToken, err := a.accountAccessToken()
	if err != nil {
		return ToolOptionsResponse{}, err
	}

	data, _, err := a.dashboardData(http.MethodGet, "/api/user/self/groups", accessToken, nil)
	if err != nil {
		return ToolOptionsResponse{}, err
	}
	groups := map[string]dashboardGroup{}
	if err := json.Unmarshal(data, &groups); err != nil {
		return ToolOptionsResponse{}, errors.New("词元神分组数据格式无法识别")
	}

	result := ToolOptionsResponse{ClientID: clientID, Groups: make([]ToolGroupOption, 0, len(groups))}
	for groupName, group := range groups {
		if groupName == "" || groupName == "auto" {
			continue
		}
		models, err := a.fetchDashboardGroupModels(accessToken, groupName)
		if err != nil {
			continue
		}
		models = filterModelsForClient(clientID, models)
		if len(models) == 0 {
			continue
		}
		result.Groups = append(result.Groups, ToolGroupOption{
			Name:        groupName,
			Description: group.Description,
			Ratio:       formatDashboardRatio(group.Ratio),
			Models:      models,
		})
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].Name < result.Groups[j].Name })
	if includeExisting {
		candidates, candidatesErr := a.findExistingToolKeys(accessToken, clientID)
		if candidatesErr != nil {
			return ToolOptionsResponse{}, fmt.Errorf("检测账号已有 Key 失败：%w", candidatesErr)
		}
		result.ExistingKeys = candidates
	}
	if len(result.Groups) == 0 && len(result.ExistingKeys) == 0 {
		return ToolOptionsResponse{}, fmt.Errorf("没有找到支持 %s 的可用分组或已有 Key", clientDisplayName(clientID))
	}
	return result, nil
}

func (a *App) CreateToolKey(request ToolKeyRequest) (ToolKeyResult, error) {
	clientID, err := normaliseClientID(request.ClientID)
	if err != nil {
		return ToolKeyResult{}, err
	}
	groupName := strings.TrimSpace(request.Group)
	if groupName == "" {
		return ToolKeyResult{}, errors.New("请选择分组")
	}
	accessToken, err := a.accountAccessToken()
	if err != nil {
		return ToolKeyResult{}, err
	}

	options, err := a.accountToolOptions(clientID, false)
	if err != nil {
		return ToolKeyResult{}, err
	}
	var selected *ToolGroupOption
	for index := range options.Groups {
		if options.Groups[index].Name == groupName {
			selected = &options.Groups[index]
			break
		}
	}
	if selected == nil {
		return ToolKeyResult{}, errors.New("所选分组不支持该工具或已不可用")
	}

	modelNames := make([]string, 0, len(selected.Models))
	for _, model := range selected.Models {
		modelNames = append(modelNames, model.ID)
	}
	before, err := a.fetchAccountTokens(accessToken)
	if err != nil {
		return ToolKeyResult{}, fmt.Errorf("读取账号已有 Key 失败：%w", err)
	}
	knownIDs := make(map[int]bool, len(before))
	for _, token := range before {
		knownIDs[token.ID] = true
	}
	createPayload := map[string]any{
		"name":                 automaticToolKeyName,
		"remain_quota":         0,
		"expired_time":         -1,
		"unlimited_quota":      true,
		"model_limits_enabled": true,
		"model_limits":         strings.Join(modelNames, ","),
		"allow_ips":            "",
		"group":                groupName,
		"auto_groups":          []string{},
		"cross_group_retry":    false,
	}
	data, _, err := a.dashboardData(http.MethodPost, "/api/token/", accessToken, createPayload)
	if err != nil {
		return ToolKeyResult{}, fmt.Errorf("创建 API Key 失败：%w", err)
	}
	createdID := tokenIDFromResponse(data)
	if createdID <= 0 {
		createdID, err = a.findNewTokenID(accessToken, knownIDs)
		if err != nil {
			return ToolKeyResult{}, fmt.Errorf("API Key 已创建，但读取新 Key 失败：%w", err)
		}
	}

	key, err := a.fetchAccountTokenKey(accessToken, createdID)
	if err != nil {
		return ToolKeyResult{}, fmt.Errorf("API Key 已创建，但读取新建 Key 失败：%w", err)
	}

	validated, validationErr := a.validateToolKey(clientID, key)
	if validationErr != nil {
		_, _, _ = a.dashboardData(http.MethodDelete, fmt.Sprintf("/api/token/%d", createdID), accessToken, nil)
		return ToolKeyResult{}, fmt.Errorf("新建 API Key 检测失败，已自动删除：%w", validationErr)
	}

	return a.storeProvisionedToolKey(clientID, groupName, automaticToolKeyName, key, validated, false)
}

func (a *App) fetchAccountTokens(accessToken string) ([]dashboardToken, error) {
	allTokens := make([]dashboardToken, 0)
	seenIDs := make(map[int]bool)
	for pageNumber := 1; pageNumber <= dashboardTokenMaxPages; pageNumber++ {
		path := fmt.Sprintf("/api/token/?p=%d&size=%d", pageNumber, dashboardTokenPageSize)
		data, _, err := a.dashboardData(http.MethodGet, path, accessToken, nil)
		if err != nil {
			return nil, err
		}

		var page struct {
			Items    json.RawMessage `json:"items"`
			PageSize int             `json:"page_size"`
			Total    int             `json:"total"`
		}
		if err := json.Unmarshal(data, &page); err == nil && page.Items != nil {
			var pageItems []dashboardToken
			if err := json.Unmarshal(page.Items, &pageItems); err != nil {
				return nil, errors.New("词元神 Key 列表数据格式无法识别")
			}
			for _, token := range pageItems {
				if token.ID > 0 && !seenIDs[token.ID] {
					seenIDs[token.ID] = true
					allTokens = append(allTokens, token)
				}
			}
			pageSize := page.PageSize
			if pageSize <= 0 {
				pageSize = dashboardTokenPageSize
			}
			if len(pageItems) == 0 || (page.Total > 0 && len(allTokens) >= page.Total) || len(pageItems) < pageSize {
				return allTokens, nil
			}
			continue
		}

		// Keep compatibility with older deployments that returned a bare array.
		if pageNumber != 1 {
			return nil, errors.New("词元神 Key 列表数据格式无法识别")
		}
		var tokens []dashboardToken
		if err := json.Unmarshal(data, &tokens); err != nil {
			return nil, errors.New("词元神 Key 列表数据格式无法识别")
		}
		return tokens, nil
	}
	return nil, errors.New("词元神 Key 列表分页超过安全上限")
}

func (a *App) fetchAccountTokenKey(accessToken string, tokenID int) (string, error) {
	if tokenID <= 0 {
		return "", errors.New("新建 Key 缺少有效编号")
	}
	data, _, err := a.dashboardData(http.MethodPost, fmt.Sprintf("/api/token/%d/key", tokenID), accessToken, nil)
	if err != nil {
		return "", err
	}
	var payload struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || strings.TrimSpace(payload.Key) == "" {
		return "", errors.New("响应中未返回有效密钥")
	}
	return strings.TrimSpace(payload.Key), nil
}

func (a *App) fetchAccountTokenKeys(accessToken string, tokenIDs []int) (map[int]string, error) {
	keys := make(map[int]string, len(tokenIDs))
	for start := 0; start < len(tokenIDs); start += dashboardTokenPageSize {
		end := start + dashboardTokenPageSize
		if end > len(tokenIDs) {
			end = len(tokenIDs)
		}
		data, _, err := a.dashboardData(http.MethodPost, "/api/token/batch/keys", accessToken, map[string][]int{"ids": tokenIDs[start:end]})
		if err != nil {
			return nil, err
		}
		var payload struct {
			Keys map[string]string `json:"keys"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, errors.New("批量读取 Key 响应格式无法识别")
		}
		for rawID, key := range payload.Keys {
			id, idErr := strconv.Atoi(rawID)
			if idErr != nil || id <= 0 || strings.TrimSpace(key) == "" {
				continue
			}
			keys[id] = strings.TrimSpace(key)
		}
	}
	return keys, nil
}

func tokenIDFromResponse(data json.RawMessage) int {
	var payload struct {
		ID int `json:"id"`
	}
	if json.Unmarshal(data, &payload) == nil && payload.ID > 0 {
		return payload.ID
	}
	return 0
}

func (a *App) findNewTokenID(accessToken string, knownIDs map[int]bool) (int, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		tokens, err := a.fetchAccountTokens(accessToken)
		if err != nil {
			lastErr = err
		} else {
			candidateID := 0
			candidateCreated := int64(0)
			for _, token := range tokens {
				if token.ID <= 0 || knownIDs[token.ID] || strings.TrimSpace(token.Name) != automaticToolKeyName {
					continue
				}
				if token.CreatedTime > candidateCreated || (token.CreatedTime == candidateCreated && token.ID > candidateID) {
					candidateID = token.ID
					candidateCreated = token.CreatedTime
				}
			}
			if candidateID > 0 {
				return candidateID, nil
			}
			lastErr = errors.New("新建 Key 尚未出现在账号列表中")
		}
		if attempt < 3 {
			time.Sleep(150 * time.Millisecond)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("未找到新建 Key")
	}
	return 0, lastErr
}

func (a *App) findExistingToolKeys(accessToken, clientID string) ([]ToolKeyResult, error) {
	tokens, err := a.fetchAccountTokens(accessToken)
	if err != nil {
		return nil, err
	}
	results := make([]ToolKeyResult, 0, len(tokens))
	available := make([]dashboardToken, 0, len(tokens))
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token.ID <= 0 || !dashboardTokenAvailable(token) {
			continue
		}
		available = append(available, token)
		ids = append(ids, token.ID)
	}
	keys, err := a.fetchAccountTokenKeys(accessToken, ids)
	if err != nil {
		return nil, err
	}
	for _, token := range available {
		key := keys[token.ID]
		if key == "" {
			continue
		}
		validated, validateErr := a.validateToolKey(clientID, key)
		if validateErr != nil {
			continue
		}
		result, provisionErr := a.storeProvisionedToolKey(clientID, strings.TrimSpace(token.Group), strings.TrimSpace(token.Name), key, validated, true)
		if provisionErr != nil {
			return nil, provisionErr
		}
		result.Existing = true
		results = append(results, result)
	}
	return results, nil
}

func dashboardTokenAvailable(token dashboardToken) bool {
	if token.Status != 1 {
		return false
	}
	if token.ExpiredTime >= 0 && token.ExpiredTime <= time.Now().Unix() {
		return false
	}
	return token.UnlimitedQuota || token.RemainQuota > 0
}

func (a *App) storeProvisionedToolKey(clientID, group, name, key string, validated ToolKeyValidationResult, existing bool) (ToolKeyResult, error) {
	provisionID, err := createProvisionID()
	if err != nil {
		return ToolKeyResult{}, errors.New("生成本地配置会话失败")
	}
	a.provisionMu.Lock()
	a.pruneProvisionsLocked()
	a.provisions[provisionID] = provisionedToolKey{
		ClientID: clientID,
		Group:    group,
		Name:     name,
		Key:      key,
		Models:   validated.Models,
		Existing: existing,
		Created:  time.Now(),
	}
	a.provisionMu.Unlock()

	return ToolKeyResult{
		ProvisionID: provisionID,
		ClientID:    clientID,
		Group:       group,
		Name:        name,
		Existing:    existing,
		Models:      validated.Models,
		Status:      validated.Status,
		Endpoint:    validated.Endpoint,
	}, nil
}

func (a *App) ValidateToolKey(request ToolKeyValidationRequest) (ToolKeyValidationResult, error) {
	clientID, err := normaliseClientID(request.ClientID)
	if err != nil {
		return ToolKeyValidationResult{}, err
	}
	return a.validateToolKey(clientID, request.APIKey)
}

func (a *App) GetConfiguredToolModels(clientID string) (ToolKeyValidationResult, error) {
	clientID, err := normaliseClientID(clientID)
	if err != nil {
		return ToolKeyValidationResult{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ToolKeyValidationResult{}, errors.New("无法确定当前用户目录")
	}
	key, err := readConfiguredClientAPIKey(home, clientID)
	if err != nil {
		return ToolKeyValidationResult{}, err
	}
	result, err := a.validateToolKey(clientID, key)
	if err != nil {
		return ToolKeyValidationResult{}, err
	}
	if selected, selectedErr := readConfiguredClientDefaultModel(home, clientID); selectedErr == nil {
		result.SelectedModel = selected
	}
	return result, nil
}

func (a *App) ConfigureTool(request ToolConfigurationRequest) ConfigureResult {
	clientID, err := normaliseClientID(request.ClientID)
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: err.Error()}
	}
	validated, err := a.validateToolKey(clientID, request.APIKey)
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: err.Error()}
	}
	if !containsModel(validated.Models, request.Model) {
		return ConfigureResult{FinishedAt: time.Now(), Error: "请选择该 Key 可用的默认模型"}
	}
	return a.Configure(ConfigurationRequest{
		APIKey:  strings.TrimSpace(request.APIKey),
		Targets: []string{clientID},
		Models:  map[string]string{clientID: strings.TrimSpace(request.Model)},
	})
}

func (a *App) ConfigureProvisionedTool(request ProvisionedToolConfigurationRequest) ConfigureResult {
	clientID, err := normaliseClientID(request.ClientID)
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: err.Error()}
	}
	a.provisionMu.Lock()
	a.pruneProvisionsLocked()
	provision, ok := a.provisions[strings.TrimSpace(request.ProvisionID)]
	a.provisionMu.Unlock()
	if !ok || provision.ClientID != clientID {
		return ConfigureResult{FinishedAt: time.Now(), Error: "配置会话已过期，请重新创建 Key"}
	}
	if !containsModel(provision.Models, request.Model) {
		return ConfigureResult{FinishedAt: time.Now(), Error: "请选择新建 Key 可用的默认模型"}
	}
	result := a.Configure(ConfigurationRequest{
		APIKey:  provision.Key,
		Targets: []string{clientID},
		Models:  map[string]string{clientID: strings.TrimSpace(request.Model)},
	})
	if result.Success {
		a.provisionMu.Lock()
		delete(a.provisions, strings.TrimSpace(request.ProvisionID))
		a.provisionMu.Unlock()
	}
	return result
}

func (a *App) ConfigureExistingTool(request ExistingToolConfigurationRequest) ConfigureResult {
	clientID, err := normaliseClientID(request.ClientID)
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: err.Error()}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: "无法确定当前用户目录"}
	}
	key, err := readConfiguredClientAPIKey(home, clientID)
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: err.Error()}
	}
	validated, err := a.validateToolKey(clientID, key)
	if err != nil {
		return ConfigureResult{FinishedAt: time.Now(), Error: err.Error()}
	}
	if !containsModel(validated.Models, request.Model) {
		return ConfigureResult{FinishedAt: time.Now(), Error: "请选择当前 Key 可用的默认模型"}
	}
	return a.Configure(ConfigurationRequest{
		APIKey:  key,
		Targets: []string{clientID},
		Models:  map[string]string{clientID: strings.TrimSpace(request.Model)},
	})
}

func (a *App) validateToolKey(clientID, apiKey string) (ToolKeyValidationResult, error) {
	response, err := a.fetchGatewayModels(apiKey)
	if err != nil {
		return ToolKeyValidationResult{}, err
	}
	models := filterModelsForClient(clientID, response.Models)
	if len(models) == 0 {
		return ToolKeyValidationResult{}, fmt.Errorf("该 Key 没有可供 %s 使用的模型", clientDisplayName(clientID))
	}
	return ToolKeyValidationResult{ClientID: clientID, Models: models, Status: response.Status, Endpoint: response.Endpoint}, nil
}

func (a *App) fetchGatewayModels(apiKey string) (ModelResponse, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ModelResponse{}, errors.New("请先输入 API Key")
	}
	request, err := http.NewRequest(http.MethodGet, defaultGatewayURL+"/models", nil)
	if err != nil {
		return ModelResponse{}, errors.New("无法创建模型请求")
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ciyuanshen-config-assistant")
	client := a.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("无法连接词元神网关：%w", err)
	}
	defer response.Body.Close()
	payload, err := decodeModelPayload(response.Body)
	if err != nil {
		return ModelResponse{}, errors.New("网关返回的数据格式无法识别")
	}
	payload.Status = response.StatusCode
	payload.Endpoint = defaultGatewayURL + "/models"
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode == http.StatusUnauthorized {
			payload.Message = "API Key 无效或已过期"
		} else if payload.Message == "" {
			payload.Message = fmt.Sprintf("网关返回 HTTP %d", response.StatusCode)
		}
		return payload, errors.New(payload.Message)
	}
	if len(payload.Models) == 0 {
		return payload, errors.New("Key 可用，但网关没有返回可用模型")
	}
	return payload, nil
}

func (a *App) consumeLoginResponse(raw []byte, fallbackUsername string) (AccountLoginResult, error) {
	var envelope dashboardEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return AccountLoginResult{}, errors.New("词元神登录响应格式无法识别")
	}
	if !envelope.Success {
		if strings.TrimSpace(envelope.Message) == "" {
			return AccountLoginResult{}, errors.New("词元神登录失败")
		}
		return AccountLoginResult{}, errors.New(envelope.Message)
	}
	var payload struct {
		RequiresTwoFactor bool   `json:"require_2fa"`
		FlowToken         string `json:"flow_token"`
		ExpiresAt         int64  `json:"access_expires_at"`
		AccessToken       string `json:"access_token"`
		User              struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return AccountLoginResult{}, errors.New("词元神登录数据格式无法识别")
	}
	if payload.RequiresTwoFactor {
		if strings.TrimSpace(payload.FlowToken) == "" {
			return AccountLoginResult{}, errors.New("两步验证会话无效")
		}
		return AccountLoginResult{RequiresTwoFactor: true, FlowToken: payload.FlowToken, Username: fallbackUsername}, nil
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return AccountLoginResult{}, errors.New("登录后未获得有效会话")
	}
	username := strings.TrimSpace(payload.User.Username)
	if username == "" {
		username = fallbackUsername
	}
	expiresAt := time.Time{}
	if payload.ExpiresAt > 0 {
		expiresAt = time.Unix(payload.ExpiresAt, 0)
	}
	a.accountMu.Lock()
	a.account = dashboardSession{AccessToken: payload.AccessToken, Username: username, ExpiresAt: expiresAt}
	a.accountMu.Unlock()
	return AccountLoginResult{SignedIn: true, Username: username, ExpiresAt: expiresAt}, nil
}

func (a *App) requestAltchaToken() (string, error) {
	_, raw, err := a.dashboardRaw(http.MethodGet, "/api/captcha/altcha/challenge", "", nil)
	if err != nil {
		return "", err
	}
	challenge := altchaChallenge{}
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return "", errors.New("ALTCHA 验证数据格式无效")
	}
	return solveAltchaChallenge(challenge)
}

func solveAltchaChallenge(challenge altchaChallenge) (string, error) {
	if challenge.Algorithm != "SHA-256" || challenge.MaxNumber <= 0 || challenge.MaxNumber > maximumAltchaAttempts || challenge.Salt == "" || challenge.Challenge == "" || challenge.Signature == "" {
		return "", errors.New("ALTCHA 验证数据无效")
	}
	needle := strings.ToLower(strings.TrimSpace(challenge.Challenge))
	for number := 0; number < challenge.MaxNumber; number++ {
		sum := sha256.Sum256([]byte(challenge.Salt + strconv.Itoa(number)))
		if hex.EncodeToString(sum[:]) != needle {
			continue
		}
		payload, err := json.Marshal(altchaPayload{
			Algorithm: challenge.Algorithm,
			Challenge: challenge.Challenge,
			Number:    number,
			Salt:      challenge.Salt,
			Signature: challenge.Signature,
		})
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(payload), nil
	}
	return "", errors.New("ALTCHA 验证计算超时")
}

func (a *App) fetchDashboardGroupModels(accessToken, groupName string) ([]Model, error) {
	data, _, err := a.dashboardData(http.MethodGet, "/api/user/models?group="+url.QueryEscape(groupName), accessToken, nil)
	if err != nil {
		return nil, err
	}
	return decodeDashboardModels(data)
}

func decodeDashboardModels(raw json.RawMessage) ([]Model, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err == nil {
		models := make([]Model, 0, len(names))
		for _, name := range names {
			if name = strings.TrimSpace(name); name != "" {
				models = append(models, Model{ID: name})
			}
		}
		return uniqueModels(models), nil
	}
	var models []Model
	if err := json.Unmarshal(raw, &models); err != nil {
		return nil, errors.New("词元神模型数据格式无法识别")
	}
	return uniqueModels(models), nil
}

func filterModelsForClient(clientID string, models []Model) []Model {
	filtered := make([]Model, 0, len(models))
	for _, model := range uniqueModels(models) {
		name := strings.ToLower(strings.TrimSpace(model.ID))
		matches := false
		switch clientID {
		case "claude":
			matches = strings.HasPrefix(name, "claude")
		case "claude-desktop":
			matches = isClaudeDesktopCompatibleModel(name)
		case "codex":
			matches = strings.HasPrefix(name, "gpt")
		case "gemini":
			matches = strings.HasPrefix(name, "gemini")
		case "grok":
			matches = strings.HasPrefix(name, "grok")
		case "opencode", "openclaw", "hermes":
			matches = name != ""
		}
		if matches {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func uniqueModels(models []Model) []Model {
	seen := map[string]bool{}
	unique := make([]Model, 0, len(models))
	for _, model := range models {
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" || seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		unique = append(unique, model)
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].ID < unique[j].ID })
	return unique
}

func containsModel(models []Model, value string) bool {
	value = strings.TrimSpace(value)
	for _, model := range models {
		if model.ID == value {
			return true
		}
	}
	return false
}

func normaliseClientID(value string) (string, error) {
	clientID := strings.ToLower(strings.TrimSpace(value))
	for _, definition := range clientDefinitions() {
		if definition.ID == clientID {
			return clientID, nil
		}
	}
	return "", fmt.Errorf("不支持的工具：%s", value)
}

func formatDashboardRatio(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return strings.TrimSpace(text)
	}
	var number float64
	if json.Unmarshal(value, &number) == nil {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return ""
}

func createProvisionID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (a *App) accountAccessToken() (string, error) {
	a.accountMu.RLock()
	session := a.account
	a.accountMu.RUnlock()
	if session.AccessToken == "" {
		return "", errors.New("请先登录词元神账号")
	}
	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		a.clearAccountSession()
		return "", errors.New("登录状态已过期，请重新登录")
	}
	return session.AccessToken, nil
}

func (a *App) clearAccountSession() {
	a.accountMu.Lock()
	a.account = dashboardSession{}
	a.accountMu.Unlock()
	a.provisionMu.Lock()
	a.provisions = map[string]provisionedToolKey{}
	a.provisionMu.Unlock()
}

func (a *App) pruneProvisionsLocked() {
	deadline := time.Now().Add(-provisionLifetime)
	for id, provision := range a.provisions {
		if provision.Created.Before(deadline) {
			delete(a.provisions, id)
		}
	}
}

func (a *App) dashboardData(method, path, accessToken string, payload any) (json.RawMessage, int, error) {
	status, raw, err := a.dashboardRaw(method, path, accessToken, payload)
	if err != nil {
		if status == http.StatusUnauthorized {
			a.clearAccountSession()
			return nil, status, errors.New("登录状态已过期，请重新登录")
		}
		return nil, status, err
	}
	var envelope dashboardEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, status, errors.New("词元神响应格式无法识别")
	}
	if !envelope.Success {
		if strings.TrimSpace(envelope.Message) == "" {
			return nil, status, errors.New("词元神请求失败")
		}
		return nil, status, errors.New(envelope.Message)
	}
	return envelope.Data, status, nil
}

func (a *App) dashboardRaw(method, path, accessToken string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, errors.New("无法编码词元神请求")
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, dashboardBaseURL+path, body)
	if err != nil {
		return 0, nil, errors.New("无法创建词元神请求")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "ciyuanshen-config-assistant")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(accessToken) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	}
	client := a.client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("无法连接词元神：%w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return response.StatusCode, nil, errors.New("读取词元神响应失败")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := responseMessage(content)
		if message == "" {
			message = fmt.Sprintf("词元神返回 HTTP %d", response.StatusCode)
		}
		return response.StatusCode, content, errors.New(message)
	}
	return response.StatusCode, content, nil
}
