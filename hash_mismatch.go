package main

import (
	"errors"
	"regexp"
	"strings"
)

type HashMismatchResult struct {
	Specified, Got string
}

var (
	gotRe       = regexp.MustCompile("^.*got:\\s*(.*)\\s*$")
	specifiedRe = regexp.MustCompile("^.*specified:\\s*(.*)\\s*$")
)

func FindHashMismatch(buildOutput string) (HashMismatchResult, error) {
	lines := strings.Split(buildOutput, "\n")
	if len(lines) < 3 {
		return HashMismatchResult{}, errors.New("output too small")
	}
	mismatchLine := -1
	for i, line := range lines {
		if strings.Contains(line, "hash mismatch in fixed-output derivation") {
			mismatchLine = i
			break
		}
	}
	if mismatchLine == -1 {
		return HashMismatchResult{}, errors.New("no hash mismatch message found")
	}
	gotLine := lines[mismatchLine+2]
	specifiedLine := lines[mismatchLine+1]

	gotMatches := gotRe.FindStringSubmatch(gotLine)
	if len(gotMatches) != 2 {
		return HashMismatchResult{}, errors.New("hash mismatch parse error")
	}
	gotHash := gotMatches[1]

	specifiedMatches := specifiedRe.FindStringSubmatch(specifiedLine)
	if len(specifiedMatches) != 2 {
		return HashMismatchResult{}, errors.New("hash mismatch parse error")
	}
	specifiedHash := specifiedMatches[1]

	return HashMismatchResult{Got: gotHash, Specified: specifiedHash}, nil
}
