// Package testenv provides shared test setup helpers.
package testenv

import (
	"os"
	"path/filepath"
)

// UnsetGitEnv clears environment variables that git sets when running hooks,
// and that would otherwise override -C or os.Chdir in subprocess calls made
// by tests. It also points GIT_TEMPLATE_DIR at an empty directory so that
// the user's hook templates don't install into test repos.
func UnsetGitEnv(emptyTemplateDir string) {
	for _, k := range []string{
		"GIT_DIR",
		"GIT_INDEX_FILE",
		"GIT_WORK_TREE",
		"GIT_COMMON_DIR",
		"GIT_OBJECT_DIRECTORY",
		"GIT_PREFIX",
	} {
		os.Unsetenv(k)
	}
	if emptyTemplateDir == "" {
		emptyTemplateDir = filepath.Join(os.TempDir(), "vcs-test-empty-template")
	}
	os.MkdirAll(emptyTemplateDir, 0755)
	os.Setenv("GIT_TEMPLATE_DIR", emptyTemplateDir)
}
