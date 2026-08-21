package main

import (
	"slices"
	"strings"
	"testing"
)

// This page is public: anything printed on it is printed to the world. These
// tests exist so a change to the masking rules cannot quietly start leaking a
// password — they are the reason the `test` gate is worth having on this repo.

func TestMaskValueHidesSensitiveVariablesEntirely(t *testing.T) {
	for _, name := range []string{
		"API_KEY", "api_key", "DB_PASSWORD", "SESSION_TOKEN", "MY_SECRET", "APIKEY",
	} {
		got := maskValue(name, "hunter2")
		if strings.Contains(got, "hunter2") {
			t.Errorf("maskValue(%q, …) = %q, which still contains the value", name, got)
		}
	}
}

func TestMaskValueStripsThePasswordOutOfAConnectionURL(t *testing.T) {
	// DATABASE_URL is injected by the postgresql add-on and is not "sensitive"
	// by name, so only the URL rule stands between its password and the page.
	const dsn = "postgresql://app:s3cr3t@demo-postgresql-rw:5432/app"
	got := maskValue("DATABASE_URL", dsn)

	if strings.Contains(got, "s3cr3t") {
		t.Fatalf("maskValue(DATABASE_URL, …) = %q, password not masked", got)
	}
	// The rest has to survive: masking the whole value would make the page
	// useless for checking which database the app actually reached.
	for _, want := range []string{"postgresql://app:", "demo-postgresql-rw:5432/app"} {
		if !strings.Contains(got, want) {
			t.Errorf("maskValue(DATABASE_URL, …) = %q, lost %q", got, want)
		}
	}
}

func TestMaskValueLeavesOrdinaryValuesAlone(t *testing.T) {
	for _, value := range []string{"debug", "8080", "https://example.com/health"} {
		if got := maskValue("LOG_LEVEL", value); got != value {
			t.Errorf("maskValue(LOG_LEVEL, %q) = %q, want it untouched", value, got)
		}
	}
}

// envLines drops the variables kubelet injects for every Service in the
// namespace. Without that filter the page is a wall of noise and the variables
// the operator injected — the ones the reader came for — are lost in it.
func TestEnvLinesSkipsKubernetesMachinery(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	t.Setenv("DEMO_SERVICE_PORT", "80")
	t.Setenv("DEMO_PORT_80_TCP_ADDR", "10.0.0.2")
	t.Setenv("GREETING", "bonjour")

	lines := envLines()
	for _, line := range lines {
		name, _, _ := strings.Cut(line, "=")
		if strings.HasPrefix(name, "KUBERNETES_") || serviceLinkVar.MatchString(name) {
			t.Errorf("envLines() kept the machinery variable %q", name)
		}
	}
	if !slices.Contains(lines, "GREETING=bonjour") {
		t.Errorf("envLines() dropped an application variable: %v", lines)
	}
}

func TestEnvLinesIsSorted(t *testing.T) {
	t.Setenv("ZZZ_LAST", "z")
	t.Setenv("AAA_FIRST", "a")

	lines := envLines()
	if !slices.IsSorted(lines) {
		t.Errorf("envLines() is not sorted: %v", lines)
	}
}

// A token carried in a URL query string is not masked today: sensitiveName
// only looks at the variable's *name*, and urlPassword only matches the
// user:password@host form. So CALLBACK_URL=https://…?access_token=… prints the
// token in full on a public page.
//
// This test states the requirement rather than the current behaviour, so it
// fails — which is the point: it is what the `test` gate is meant to catch
// before such a commit reaches production.
func TestMaskValueHidesATokenPassedInAQueryString(t *testing.T) {
	const callback = "https://example.com/callback?access_token=s3cr3t&state=x"
	got := maskValue("CALLBACK_URL", callback)

	if strings.Contains(got, "s3cr3t") {
		t.Errorf("maskValue(CALLBACK_URL, …) = %q, the token is printed in full", got)
	}
}
