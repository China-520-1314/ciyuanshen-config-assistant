package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	home := isolateHome(t)
	backupConfigHome := filepath.Join(home, "config")
	t.Setenv("XDG_CONFIG_HOME", backupConfigHome)

	existing := filepath.Join(home, "settings.json")
	missing := filepath.Join(home, "new.toml")
	writeFixture(t, existing, "before")
	operations := []fileOperation{
		{ClientID: "test", Path: existing, Content: []byte("after")},
		{ClientID: "test", Path: missing, Content: []byte("created")},
	}

	backup, err := createBackup(operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(backup.Files) != 2 || !backup.Files[0].Exists || backup.Files[1].Exists {
		t.Fatalf("unexpected backup entries: %#v", backup.Files)
	}
	writeFixture(t, existing, "changed")
	writeFixture(t, missing, "created")

	if err := restoreBackup(backup); err != nil {
		t.Fatal(err)
	}
	existingContent, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(existingContent) != "before" {
		t.Fatalf("existing file was not restored: %q", existingContent)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("new file should have been removed, stat error: %v", err)
	}

	backups, err := listBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0].ID != backup.ID {
		t.Fatalf("backup was not listed: %#v", backups)
	}
}

func TestBackupIDValidation(t *testing.T) {
	for _, id := range []string{"", ".", "..", "../escape", `..\escape`, "/absolute"} {
		if isSafeBackupID(id) {
			t.Fatalf("backup ID %q should be rejected", id)
		}
	}
	for _, id := range []string{"20260818-120000.123", "backup-1"} {
		if !isSafeBackupID(id) {
			t.Fatalf("backup ID %q should be accepted", id)
		}
	}
}

func TestDeleteBackupByID(t *testing.T) {
	home := isolateHome(t)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	path := filepath.Join(home, "settings.json")
	writeFixture(t, path, "before")
	backup, err := createBackup([]fileOperation{{ClientID: "test", Path: path, Content: []byte("after")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteBackupByID(backup.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup.Path); !os.IsNotExist(err) {
		t.Fatalf("backup directory should be deleted, stat error: %v", err)
	}
	backups, err := listBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("deleted backup was still listed: %#v", backups)
	}
	if err := deleteBackupByID("../outside"); err == nil {
		t.Fatal("unsafe backup ID should be rejected")
	}
}
