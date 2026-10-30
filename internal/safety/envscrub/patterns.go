package envscrub

import "strings"

// denyPatterns are credential variable names (upper-case globs: * is any
// run of characters). They come from docs/04-safety.md Layer 3, plus the
// credential variables of the tools the docs name.
var denyPatterns = []string{
	// Cloud providers
	"AWS_*",
	"GOOGLE_APPLICATION_CREDENTIALS", "GOOGLE_*CREDENTIALS*", "GCLOUD_*KEY*", "CLOUDSDK_AUTH_*",
	"AZURE_*SECRET*", "AZURE_CLIENT_*", "AZURE_PASSWORD", "AZURE_DEVOPS_EXT_PAT", "ARM_CLIENT_*", "ARM_ACCESS_KEY",
	// Source hosting and package registries
	"GH_TOKEN", "GH_*TOKEN*", "GITHUB_TOKEN", "GITHUB_*TOKEN*", "GITLAB_*TOKEN*", "CI_JOB_TOKEN", "BITBUCKET_*TOKEN*",
	"NPM_TOKEN", "NPM_CONFIG_*AUTH*", "NODE_AUTH_TOKEN", "YARN_NPM_AUTH_*",
	"CARGO_REGISTRY_TOKEN", "CARGO_REGISTRIES_*_TOKEN", "PYPI_TOKEN", "TWINE_*", "POETRY_PYPI_TOKEN_*", "POETRY_HTTP_BASIC_*",
	// Clusters, databases, secret stores
	"KUBECONFIG", "DATABASE_URL", "*_DATABASE_URL", "*_DB_URL", "MONGODB_URI", "MONGO_URL",
	"REDIS_URL", "*_REDIS_URL", "PGPASSWORD", "PGPASSFILE", "MYSQL_PWD",
	"DOCKER_AUTH_CONFIG", "DOCKER_PASSWORD", "VAULT_TOKEN", "VAULT_*TOKEN*", "TF_TOKEN_*",
	"OP_SESSION_*", "BW_*",
	// Agents and askpass helpers that could hand out keys
	"SSH_AUTH_SOCK", "SSH_AGENT_PID", "GPG_AGENT_INFO", "GIT_ASKPASS", "SSH_ASKPASS", "SUDO_ASKPASS",
	// Generic names
	"TOKEN", "SECRET", "PASSWORD", "API_KEY", "APIKEY",
	"*_TOKEN", "*_SECRET", "*_PASSWORD", "*_PASSWD", "*_PASSPHRASE", "*_API_KEY", "*_APIKEY",
	"*_ACCESS_KEY", "*_SECRET_KEY", "*_PRIVATE_KEY", "*_CREDENTIALS", "*_CREDENTIAL", "*_AUTH",
}

// builtinKeep are the agent CLIs' own variables. Claude Code needs
// ANTHROPIC_API_KEY or its OAuth token to run at all, so these are kept.
// A user can still remove them with ExtraDeny.
var builtinKeep = []string{"ANTHROPIC_*", "CLAUDE_*", "CLAUDECODE"}

// glob matches s against pat where * is any run of characters.
func glob(pat, s string) bool {
	px, sx := 0, 0
	star, mark := -1, 0
	for sx < len(s) {
		switch {
		case px < len(pat) && pat[px] == s[sx]:
			px++
			sx++
		case px < len(pat) && pat[px] == '*':
			star, mark = px, sx
			px++
		case star >= 0:
			px = star + 1
			mark++
			sx = mark
		default:
			return false
		}
	}
	for px < len(pat) && pat[px] == '*' {
		px++
	}
	return px == len(pat)
}

func matchAny(patterns []string, upperName string) bool {
	for _, p := range patterns {
		if glob(strings.ToUpper(p), upperName) {
			return true
		}
	}
	return false
}
