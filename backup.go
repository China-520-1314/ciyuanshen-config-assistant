package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type BackupFile struct {
	ClientID     string `json:"clientId"`
	OriginalPath string `json:"originalPath"`
	BackupPath   string `json:"backupPath"`
	Exists       bool   `json:"exists"`
}

type BackupInfo struct {
	ID        string       `json:"id"`
	CreatedAt time.Time    `json:"createdAt"`
	Path      string       `json:"path"`
	Files     []BackupFile `json:"files"`
}

func backupRoot() string {
	base, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(base) == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return filepath.Join(".", ".ciyuanshen-config-assistant", "backups")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "CiyuanShen", "Config Assistant", "backups")
}

func createBackup(operations []fileOperation) (BackupInfo, error) {
	now := time.Now()
	id := now.Format("20060102-150405.000000000")
	root := filepath.Join(backupRoot(), id)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return BackupInfo{}, err
	}

	backup := BackupInfo{ID: id, CreatedAt: now, Path: root, Files: []BackupFile{}}
	seen := map[string]bool{}
	for index, operation := range operations {
		if seen[operation.Path] {
			continue
		}
		seen[operation.Path] = true
		entry := BackupFile{
			ClientID:     operation.ClientID,
			OriginalPath: operation.Path,
			BackupPath:   filepath.Join(root, fmt.Sprintf("%02d-%s", index, filepath.Base(operation.Path))),
		}
		if _, err := os.Stat(operation.Path); err == nil {
			content, readErr := os.ReadFile(operation.Path)
			if readErr != nil {
				return BackupInfo{}, readErr
			}
			if writeErr := atomicWrite(entry.BackupPath, content); writeErr != nil {
				return BackupInfo{}, writeErr
			}
			entry.Exists = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return BackupInfo{}, err
		}
		backup.Files = append(backup.Files, entry)
	}
	manifest, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return BackupInfo{}, err
	}
	if err := atomicWrite(filepath.Join(root, "manifest.json"), append(manifest, '\n')); err != nil {
		return BackupInfo{}, err
	}
	return backup, nil
}

func listBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(backupRoot())
	if errors.Is(err, os.ErrNotExist) {
		return []BackupInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	backups := make([]BackupInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !isSafeBackupID(entry.Name()) {
			continue
		}
		manifestPath := filepath.Join(backupRoot(), entry.Name(), "manifest.json")
		content, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			continue
		}
		var backup BackupInfo
		if json.Unmarshal(content, &backup) != nil {
			continue
		}
		backups = append(backups, backup)
	}
	sort.Slice(backups, func(left, right int) bool {
		return backups[left].CreatedAt.After(backups[right].CreatedAt)
	})
	return backups, nil
}

func deleteBackupByID(id string) error {
	if !isSafeBackupID(id) {
		return errors.New("无效的备份编号")
	}

	root := filepath.Clean(backupRoot())
	path := filepath.Join(root, id)
	if filepath.Dir(path) != root {
		return errors.New("无效的备份目录")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return errors.New("找不到该备份")
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("备份目录无效")
	}
	return os.RemoveAll(path)
}

func restoreBackup(backup BackupInfo) error {
	for _, entry := range backup.Files {
		if entry.Exists {
			content, err := os.ReadFile(entry.BackupPath)
			if err != nil {
				return err
			}
			if err := atomicWrite(entry.OriginalPath, content); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(entry.OriginalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func restoreBackupByID(id string) error {
	if !isSafeBackupID(id) {
		return errors.New("无效的备份编号")
	}
	manifestPath := filepath.Join(backupRoot(), id, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return errors.New("找不到该备份")
	}
	var backup BackupInfo
	if err := json.Unmarshal(content, &backup); err != nil {
		return errors.New("备份清单损坏")
	}
	return restoreBackup(backup)
}

func isSafeBackupID(id string) bool {
	return id != "" && id != "." && id != ".." && filepath.Base(id) == id && !strings.ContainsAny(id, `/\\`)
}

func atomicWrite(path string, content []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
