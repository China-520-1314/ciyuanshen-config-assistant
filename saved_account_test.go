package main

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSavedAccountLoginRoundTrip(t *testing.T) {
	keyring.MockInit()
	app := NewApp()

	if saved, err := app.GetSavedAccountLogin(); err != nil || saved.Username != "" || saved.Password != "" {
		t.Fatalf("empty keyring result = %#v, err=%v", saved, err)
	}
	if err := app.SaveAccountLogin(SavedAccountLogin{Username: " alice ", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	saved, err := app.GetSavedAccountLogin()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Username != "alice" || saved.Password != "secret" {
		t.Fatalf("saved login = %#v", saved)
	}
	if err := app.DeleteSavedAccountLogin(); err != nil {
		t.Fatal(err)
	}
	if saved, err := app.GetSavedAccountLogin(); err != nil || saved.Username != "" {
		t.Fatalf("deleted keyring result = %#v, err=%v", saved, err)
	}
}

func TestSavedAccountLoginRejectsIncompleteCredentials(t *testing.T) {
	keyring.MockInit()
	app := NewApp()
	if err := app.SaveAccountLogin(SavedAccountLogin{Username: "alice"}); err == nil {
		t.Fatal("expected incomplete credentials to be rejected")
	}
}
