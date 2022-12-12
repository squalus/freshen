package flake

import (
	"encoding/json"
	"os"
)

type Locks struct {
	Nodes map[string]LockNode `json:"nodes"`
}

type LockNode struct {
	Locked LockInfo `json:"locked"`
}

type LockInfo struct {
	Type         string `json:"type"`
	LastModified uint64 `json:"lastModified"`
	NarHash      string `json:"narHash"`
	Rev          string `json:"rev"`
}

func ReadMetadata(buf []byte) (out Locks, err error) {
	if err = json.Unmarshal(buf, &out); err != nil {
		return out, err
	}
	return out, nil
}

func ReadMetadataFile(path string) (out Locks, err error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	return ReadMetadata(buf)
}

func (m Locks) InputRev(input string) (string, bool) {
	node, ok := m.Nodes[input]
	if !ok {
		return "", false
	}
	if node.Locked.Rev == "" {
		return "", false
	}
	return node.Locked.Rev, true
}
