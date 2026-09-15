package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

func TestNativeAuthReportsIdentityWithoutExposingIt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	prependFakeExecutable(t, dir, "codex", "codex-cli test")
	path := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"auth_mode": "chatgpt",
		"tokens":    map[string]string{"account_id": "account-secret-id"},
	}
	contents, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(path, "auth.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	binding := discoverNativeCodex()
	if binding.IdentityStatus != domain.IdentityVerified {
		t.Fatalf("identity status = %q", binding.IdentityStatus)
	}
	if binding.IdentityHash == "account-secret-id" || binding.IdentityHash == "" {
		t.Fatalf("identity hash leaked or missing: %q", binding.IdentityHash)
	}
}

func TestNativeCodexDetectsEphemeralCapabilityFromHelp(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	prependFakeExecutableScript(t, dir, "codex", `#!/bin/sh
if [ "$1" = "--version" ]; then
  printf '%s\n' 'codex-cli 1.2.3'
  exit 0
fi
if [ "$1" = "exec" ] && [ "$2" = "--help" ]; then
  printf '%s\n' 'Usage: codex exec [OPTIONS] --ephemeral --sandbox <MODE>'
  exit 0
fi
exit 1
`)
	path := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	contents, _ := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens":    map[string]string{"account_id": "account-secret-id"},
	})
	if err := os.WriteFile(filepath.Join(path, "auth.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	binding := discoverNativeCodex()
	if binding.Version != "codex-cli 1.2.3" {
		t.Fatalf("version = %q", binding.Version)
	}
	if !binding.Supports(domain.CapabilityEphemeral) {
		t.Fatalf("ephemeral capability was not detected: %+v", binding.Capabilities)
	}
}

func TestHermesBindingCanBeDiscoveredFromProfileStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("HERMES_HOME", filepath.Join(dir, "profile"))
	prependFakeExecutable(t, dir, "hermes", "hermes test")
	if err := os.MkdirAll(filepath.Join(dir, "profile"), 0o700); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"credential_pool": map[string]any{
			"openai-codex": []map[string]string{{"auth_type": "oauth", "source": "device_code", "base_url": "https://chatgpt.com/backend-api/codex"}},
		},
	}
	contents, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(dir, "profile", "auth.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	binding := discoverHermesCodex()
	if !binding.Available || binding.IdentityStatus != domain.IdentityUnknown {
		t.Fatalf("unexpected binding: %+v", binding)
	}
	if binding.AuthStore != filepath.Join(dir, "profile", "auth.json") {
		t.Fatalf("auth store = %q", binding.AuthStore)
	}
}

func prependFakeExecutable(t *testing.T, dir, name, version string) {
	t.Helper()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(binDir, name)
	contents := "#!/bin/sh\nprintf '%s\\n' '" + version + "'\n"
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func prependFakeExecutableScript(t *testing.T, dir, name, contents string) {
	t.Helper()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(binDir, name)
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCommandVersionTimesOut(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "hanging-provider")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	previous := discoveryCommandTimeout
	discoveryCommandTimeout = 50 * time.Millisecond
	t.Cleanup(func() { discoveryCommandTimeout = previous })

	started := time.Now()
	if version := commandVersion(executable); version != "unknown" {
		t.Fatalf("version = %q", version)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("discovery timeout took %s", elapsed)
	}
}
