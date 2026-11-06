package qualitydup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepeatedWindowIncreasesDebt(t *testing.T) {
	source := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	a, b := Windows(source), Windows(source+source)
	if len(a) != 1 || len(b) != 11 {
		t.Fatal("window length drift")
	}
	if !Exceeds(1, 0) || Exceeds(0, 0) {
		t.Fatal("duplicate gate drift")
	}
}

func TestNewDuplicateBlockIsBlocked(t *testing.T) {
	root := t.TempDir()
	source := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	if err := os.WriteFile(filepath.Join(root, "a.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := Measure(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.ts"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := Measure(root)
	if err != nil || !Exceeds(after, before) {
		t.Fatal("duplicate production block accepted", err)
	}
}
