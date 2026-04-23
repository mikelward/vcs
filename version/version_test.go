package version

import "testing"

var sample = Info{
	Version: "v1.2.3",
	Commit:  "abc1234",
	Date:    "2024-01-02T03:04:05Z",
}

func TestInfoLine(t *testing.T) {
	want := "vcs v1.2.3 (commit abc1234, built 2024-01-02T03:04:05Z)"
	if got := sample.Line("vcs"); got != want {
		t.Errorf("Line:\n got: %q\nwant: %q", got, want)
	}
}

func TestInfoBlock(t *testing.T) {
	want := "vcs\nversion: v1.2.3\ncommit:  abc1234\nbuilt:   2024-01-02T03:04:05Z"
	if got := sample.Block("vcs"); got != want {
		t.Errorf("Block:\n got: %q\nwant: %q", got, want)
	}
}

func TestReadPopulatesAllFields(t *testing.T) {
	// Values depend on how the test binary was built, but all fields should
	// be non-empty (either a real value or the "unknown" sentinel).
	i := Read()
	if i.Version == "" || i.Commit == "" || i.Date == "" {
		t.Errorf("expected all fields populated, got %+v", i)
	}
}
