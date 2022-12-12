package flake

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
)

type Flake struct {
	Path string
}

func (f Flake) MetadataLocks() (Locks, error) {
	lockFilePath := path.Join(f.Path, "flake.lock")
	buf, err := os.ReadFile(lockFilePath)
	if err != nil {
		return Locks{}, err
	}
	return ReadMetadata(buf)
}

func (f Flake) UpdateInput(input string) error {
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("cannot find nix binary on path")
	}
	cmd := exec.Cmd{
		Path:   nixBin,
		Dir:    f.Path,
		Args:   []string{"", "flake", "lock", "--update-input", input},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("nix flake --update-input: %w", err)
	}
	return nil
}

func (f Flake) Build(attrPath string, sandbox bool) (stdout, stderr string, err error) {
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		return "", "", fmt.Errorf("cannot find nix binary on path")
	}
	var stdoutBuf, stderrBuf bytes.Buffer

	buildAttrPath := ".#" + attrPath
	fixedArgs := []string{"", "build", "-L"}
	if !sandbox {
		fixedArgs = append(fixedArgs, []string{"--option", "build-use-sandbox", "false"}...)
	}
	cmd := exec.Cmd{
		Path:   nixBin,
		Dir:    f.Path,
		Args:   append(fixedArgs, []string{buildAttrPath}...),
		Stdout: io.MultiWriter(os.Stdout, &stdoutBuf),
		Stderr: io.MultiWriter(os.Stderr, &stderrBuf),
	}
	if err = cmd.Run(); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("nix build: %w", err)
	}
	return stdoutBuf.String(), stderrBuf.String(), nil
}
