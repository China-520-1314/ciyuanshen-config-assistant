package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	savedAccountKeyringService = "ciyuanshen-config-assistant"
	savedAccountKeyringUser    = "saved-account-login"
)

// SavedAccountLogin is exchanged only when the user explicitly enables
// password remembering. The value is stored in the OS credential manager, not
// in the assistant's JSON/configuration directories.
type SavedAccountLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetSavedAccountLogin returns the remembered account, if one exists. A
// missing credential is a normal first-run state and is not an error.
func (a *App) GetSavedAccountLogin() (SavedAccountLogin, error) {
	secret, err := keyring.Get(savedAccountKeyringService, savedAccountKeyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return SavedAccountLogin{}, nil
	}
	if err != nil {
		return SavedAccountLogin{}, errors.New("无法读取系统凭据管理器中的登录信息")
	}
	var saved SavedAccountLogin
	if err := json.Unmarshal([]byte(secret), &saved); err != nil {
		return SavedAccountLogin{}, errors.New("已保存的登录信息格式无效")
	}
	saved.Username = strings.TrimSpace(saved.Username)
	if saved.Username == "" || saved.Password == "" {
		return SavedAccountLogin{}, nil
	}
	return saved, nil
}

// SaveAccountLogin stores the account credentials in the platform's secure
// credential store (Windows Credential Manager, macOS Keychain, or Secret
// Service on Linux).
func (a *App) SaveAccountLogin(saved SavedAccountLogin) error {
	saved.Username = strings.TrimSpace(saved.Username)
	if saved.Username == "" || saved.Password == "" {
		return errors.New("请输入完整的账号和密码后再保存")
	}
	encoded, err := json.Marshal(saved)
	if err != nil {
		return errors.New("无法保存登录信息")
	}
	if err := keyring.Set(savedAccountKeyringService, savedAccountKeyringUser, string(encoded)); err != nil {
		return errors.New("无法写入系统凭据管理器，请检查系统凭据服务")
	}
	return nil
}

// DeleteSavedAccountLogin removes the remembered credentials. Deleting a
// missing entry is intentionally idempotent so unchecking the option is safe
// on a fresh installation.
func (a *App) DeleteSavedAccountLogin() error {
	err := keyring.Delete(savedAccountKeyringService, savedAccountKeyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("无法删除系统凭据管理器中的登录信息")
	}
	return nil
}
