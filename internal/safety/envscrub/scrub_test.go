package envscrub

import (
	"strings"
	"testing"
)

func names(env []string) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		k, v, _ := strings.Cut(e, "=")
		m[k] = v
	}
	return m
}

func TestScrubRemovesCredentials(t *testing.T) {
	env := []string{
		"PATH=/usr/bin", "HOME=/home/u", "LANG=C", "EDITOR=vim", "TERM=xterm",
		"AWS_ACCESS_KEY_ID=AKIA", "AWS_SECRET_ACCESS_KEY=s", "AWS_REGION=us-east-1", "AWS_PROFILE=p", "AWS_SESSION_TOKEN=t",
		"GOOGLE_APPLICATION_CREDENTIALS=/k.json", "GOOGLE_CLOUD_CREDENTIALS_FILE=x", "CLOUDSDK_AUTH_ACCESS_TOKEN=t",
		"AZURE_CLIENT_ID=a", "AZURE_CLIENT_SECRET=b", "AZURE_STORAGE_SECRET_KEY=c",
		"GH_TOKEN=a", "GITHUB_TOKEN=b", "GITLAB_TOKEN=c", "GITLAB_PRIVATE_TOKEN=c", "NPM_TOKEN=d", "NODE_AUTH_TOKEN=e",
		"CARGO_REGISTRY_TOKEN=f", "PYPI_TOKEN=g", "KUBECONFIG=/k", "DATABASE_URL=postgres://u:p@h/db", "ANALYTICS_DATABASE_URL=x",
		"PGPASSWORD=p", "MYSQL_PWD=p", "REDIS_URL=redis://:pw@h", "SSH_AUTH_SOCK=/tmp/agent", "DOCKER_AUTH_CONFIG={}",
		"VAULT_TOKEN=v", "STRIPE_SECRET=s", "DB_PASSWORD=p", "OPENAI_API_KEY=k", "SENDGRID_API_KEY=k", "SLACK_BOT_TOKEN=x",
		"GIT_ASKPASS=/bin/ask", "TF_TOKEN_app_terraform_io=t", "OP_SESSION_my=x", "BW_SESSION=x",
		"github_token=lower",
	}
	res := ScrubWith(env, Options{})
	got := names(res.Env)
	for _, keep := range []string{"PATH", "HOME", "LANG", "EDITOR", "TERM"} {
		if _, ok := got[keep]; !ok {
			t.Errorf("%s was removed", keep)
		}
	}
	for _, e := range env[5:] {
		n, _, _ := strings.Cut(e, "=")
		if _, ok := got[n]; ok {
			t.Errorf("%s survived", n)
		}
	}
	if len(res.Removed) != len(env)-5 {
		t.Errorf("Removed has %d names, want %d: %v", len(res.Removed), len(env)-5, res.Removed)
	}
	for _, r := range res.Removed {
		for _, e := range res.Env {
			if strings.Contains(e, "AKIA") || strings.Contains(e, "postgres://") {
				t.Fatalf("value leaked into env: %s", e)
			}
		}
		if strings.Contains(r, "=") {
			t.Errorf("Removed contains a value: %q", r)
		}
	}
}

func TestKeepsAgentProviderVariables(t *testing.T) {
	env := []string{"ANTHROPIC_API_KEY=sk", "ANTHROPIC_AUTH_TOKEN=t", "CLAUDE_CODE_OAUTH_TOKEN=o", "CLAUDECODE=1", "OPENAI_API_KEY=k", "GEMINI_API_KEY=g", "GITHUB_TOKEN=x"}
	got := names(Scrub(env, nil))
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDECODE"} {
		if _, ok := got[k]; !ok {
			t.Errorf("%s should be kept", k)
		}
	}
	for _, k := range []string{"OPENAI_API_KEY", "GEMINI_API_KEY", "GITHUB_TOKEN"} {
		if _, ok := got[k]; ok {
			t.Errorf("%s should be removed", k)
		}
	}
	// An adapter for another CLI keeps its own provider key.
	got = names(ScrubWith(env, Options{Keep: []string{"OPENAI_API_KEY"}}).Env)
	if _, ok := got["OPENAI_API_KEY"]; !ok {
		t.Error("Keep did not keep OPENAI_API_KEY")
	}
	if _, ok := got["GEMINI_API_KEY"]; ok {
		t.Error("Keep leaked to another key")
	}
	// The user's own deny list always wins, even over built-in keeps.
	got = names(ScrubWith(env, Options{Keep: []string{"OPENAI_API_KEY"}, ExtraDeny: []string{"openai_api_key", "ANTHROPIC_API_KEY", "MY_*"}}).Env)
	if _, ok := got["OPENAI_API_KEY"]; ok {
		t.Error("ExtraDeny must beat Keep")
	}
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Error("ExtraDeny must beat the built-in keep")
	}
}

func TestExtraDenyGlob(t *testing.T) {
	got := names(Scrub([]string{"PATH=/bin", "INTERNAL_THING=1", "MY_CORP_VAR=2", "MYSELF=3"}, []string{"MY_*", "internal_thing"}))
	if _, ok := got["MY_CORP_VAR"]; ok {
		t.Error("MY_* not applied")
	}
	if _, ok := got["INTERNAL_THING"]; ok {
		t.Error("case-insensitive exact name not applied")
	}
	if _, ok := got["MYSELF"]; !ok {
		t.Error("MYSELF wrongly removed")
	}
}

func TestGitHardening(t *testing.T) {
	got := names(Scrub([]string{"PATH=/bin"}, nil))
	want := map[string]string{
		"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "credential.helper", "GIT_CONFIG_VALUE_0": "",
		"GIT_TERMINAL_PROMPT": "0", "GCM_INTERACTIVE": "never",
	}
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Errorf("%s=%q (present=%v), want %q", k, g, ok, v)
		}
	}
	// Existing config entries are preserved and the new one is appended.
	res := ScrubWith([]string{"GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=a.b", "GIT_CONFIG_VALUE_0=1", "GIT_CONFIG_KEY_1=c.d", "GIT_CONFIG_VALUE_1=2", "git_terminal_prompt=1"}, Options{})
	got = names(res.Env)
	if got["GIT_CONFIG_COUNT"] != "3" || got["GIT_CONFIG_KEY_2"] != "credential.helper" || got["GIT_CONFIG_KEY_0"] != "a.b" || got["GIT_CONFIG_KEY_1"] != "c.d" {
		t.Errorf("did not append after existing config: %v", got)
	}
	if got["GIT_TERMINAL_PROMPT"] != "0" {
		t.Error("prompt not disabled")
	}
	if _, dup := got["git_terminal_prompt"]; dup {
		t.Error("duplicate differently-cased variable kept")
	}
	if len(res.Added) != 5 {
		t.Errorf("Added = %v", res.Added)
	}
	// Garbage counts do not panic.
	Scrub([]string{"GIT_CONFIG_COUNT=zzz", "GIT_CONFIG_COUNT", "=C:=C:\\x", "NOEQUALS"}, nil)
}

func TestWindowsStyleEntries(t *testing.T) {
	got := Scrub([]string{`=C:=C:\work`, `Path=C:\bin`, `GitHub_Token=x`, `AwS_Region=eu`}, nil)
	m := names(got)
	if _, ok := m["=C:"]; !ok && !contains(got, `=C:=C:\work`) {
		t.Error("drive-cwd entry lost")
	}
	if _, ok := m["GitHub_Token"]; ok {
		t.Error("mixed-case token kept")
	}
	if _, ok := m["AwS_Region"]; ok {
		t.Error("mixed-case AWS kept")
	}
	if m["Path"] != `C:\bin` {
		t.Error("Path lost")
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func TestDoesNotMutateInput(t *testing.T) {
	in := []string{"A=1", "GITHUB_TOKEN=x", "B=2"}
	cp := append([]string(nil), in...)
	Scrub(in, nil)
	for i := range in {
		if in[i] != cp[i] {
			t.Fatal("input modified")
		}
	}
}
