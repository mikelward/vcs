package jjsync

import (
	"testing"
)

func TestParse(t *testing.T) {
	// op log is newest-first; the first sync-like line wins. The snapshot
	// line (newest) is not a sync and must be skipped.
	out := "1700000300 snapshot working copy\n" +
		"1700000200 fetch from git remote(s) origin\n" +
		"1700000100 describe commit abc\n"
	got, ok := Parse(out)
	if !ok {
		t.Fatal("Parse should find a sync operation")
	}
	if got.Unix() != 1700000200 {
		t.Errorf("Parse time = %d, want 1700000200", got.Unix())
	}
}

func TestParseVariants(t *testing.T) {
	// "Sync to CL ..." is the real description a piper pull writes.
	for _, desc := range []string{"sync workspace", "piper pull", "import commits", "Sync to CL 12345678"} {
		out := "1700000000 " + desc + "\n"
		if _, ok := Parse(out); !ok {
			t.Errorf("Parse should match %q as a sync operation", desc)
		}
	}
}

func TestParseNone(t *testing.T) {
	// "Upload ..." is a push, not a pull, so it must not count as a sync.
	out := "1700000400 Upload changes for review\n" +
		"1700000300 snapshot working copy\n" +
		"1700000100 describe commit abc\n"
	if _, ok := Parse(out); ok {
		t.Error("Parse should not match non-sync ops (snapshot/upload/describe)")
	}
	if _, ok := Parse(""); ok {
		t.Error("Parse on empty output should return false")
	}
}
