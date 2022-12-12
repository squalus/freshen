package flake

import (
	"os"
	"path"
	"testing"
)

func testdataPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(cwd, "..", "test-data")
}

func TestFlake_Metadata(t *testing.T) {
	flakeRoot := path.Join(testdataPath(), "simple-flake")
	f := Flake{Path: flakeRoot}
	meta, err := f.MetadataLocks()
	if err != nil {
		t.Fatal(err)
	}
	rev, ok := meta.InputRev("nixpkgs")
	if !ok {
		t.Fatalf("InputRev not ok")
	}
	if rev != "ffca9ffaaafb38c8979068cee98b2644bd3f14cb" {
		t.Fatalf("InputRev not equal")
	}
}
