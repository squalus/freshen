package main

import (
	"os"
	"path"
	"testing"
)

func TestFindHashMismatch(t *testing.T) {
	buf, err := os.ReadFile(path.Join("test-data", "hash-mismatch.txt"))
	if err != nil {
		t.Fatal(err)
	}
	txt := string(buf)
	result, err := FindHashMismatch(txt)
	if err != nil {
		t.Fatal(err)
	}
	if result.Got != "sha256-If5iev47iRxpVvaB7WrfV1U1xaujUsS/113cprtvaB0=" {
		t.Fatal("got hash incorrect")
	}
	if result.Specified != "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" {
		t.Fatal("specified hash incorrect")
	}
}
