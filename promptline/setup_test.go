package promptline

import (
	"os"
	"testing"

	"github.com/mikelward/vcs/internal/testenv"
)

func TestMain(m *testing.M) {
	testenv.UnsetGitEnv("")
	os.Exit(m.Run())
}
