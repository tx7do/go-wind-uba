package license

import (
	"errors"
	"testing"
	"time"
)

func TestNoopProviderDefaults(t *testing.T) {
	noop := NoopProvider{}
	if err := noop.Valid(); err != nil {
		t.Fatalf("noop Valid() = %v, want nil", err)
	}
	if noop.Allows("sso") {
		t.Fatal("noop Allows() = true, want false")
	}
	if got := noop.Licensee(); got != "" {
		t.Fatalf("noop Licensee() = %q, want empty", got)
	}
	if !noop.ExpiresAt().IsZero() {
		t.Fatal("noop ExpiresAt() not zero")
	}
}

// stubProvider 测试接缝注入行为。
type stubProvider struct {
	caps    map[string]bool
	expires time.Time
}

func (s stubProvider) Valid() error {
	if time.Now().After(s.expires) {
		return errors.New("stub expired")
	}
	return nil
}
func (s stubProvider) Allows(c string) bool { return s.caps[c] }
func (s stubProvider) Capabilities() []string {
	caps := make([]string, 0, len(s.caps))
	for c, on := range s.caps {
		if on {
			caps = append(caps, c)
		}
	}
	return caps
}
func (s stubProvider) Licensee() string     { return "stub" }
func (s stubProvider) ExpiresAt() time.Time { return s.expires }

func TestSeamInjectionAndRequire(t *testing.T) {
	t.Cleanup(func() { SetProvider(nil) })

	if _, ok := Current().(NoopProvider); !ok {
		t.Fatalf("default provider = %T, want NoopProvider", Current())
	}

	future := time.Now().Add(24 * time.Hour)
	SetProvider(stubProvider{caps: map[string]bool{"sso": true}, expires: future})

	if !Enabled("sso") {
		t.Fatal("Enabled(sso) = false after injection, want true")
	}
	if Enabled("field_permission") {
		t.Fatal("Enabled(field_permission) = true, want false")
	}
	if err := Require("sso"); err != nil {
		t.Fatalf("Require(sso) = %v, want nil", err)
	}
	var notLicensed *NotLicensedError
	if err := Require("field_permission"); !errors.As(err, &notLicensed) {
		t.Fatalf("Require(field_permission) = %v, want *NotLicensedError", err)
	}
	if Current().Licensee() != "stub" {
		t.Fatalf("Current().Licensee() = %q, want stub", Current().Licensee())
	}

	SetProvider(nil)
	if _, ok := Current().(NoopProvider); !ok {
		t.Fatalf("provider after reset = %T, want NoopProvider", Current())
	}
}
