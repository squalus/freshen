package flake

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
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

func (f Flake) BuildWithRawOutput(attrPath string, sandbox bool) (stdout, stderr string, err error) {
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

func (f Flake) Build(attrPath string) (output string, err error) {
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		return "", fmt.Errorf("cannot find nix binary on path")
	}
	var stdoutBuf bytes.Buffer

	buildAttrPath := ".#" + attrPath
	fixedArgs := []string{"", "build", "--json", "-L"}
	cmd := exec.Cmd{
		Path:   nixBin,
		Dir:    f.Path,
		Args:   append(fixedArgs, []string{buildAttrPath}...),
		Stdout: io.MultiWriter(os.Stdout, &stdoutBuf),
		Stderr: os.Stderr,
	}
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("nix build: %w", err)
	}
	stdout := strings.TrimSpace(stdoutBuf.String())
	var outJsonList []BuildOutput
	if err := json.Unmarshal([]byte(stdout), &outJsonList); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if len(outJsonList) != 1 {
		return "", errors.New("nix build json malformed: invalid root array length")
	}
	outJson := outJsonList[0]
	if outJson.Outputs == nil {
		return "", errors.New("nix build json malformed: no outputs key")
	}
	mainOutput, ok := outJson.Outputs["out"]
	if !ok {
		return "", errors.New("nix build json malformed: main output not present")
	}
	if !strings.HasPrefix(mainOutput, "/nix/store") {
		return "", errors.New("nix build sanity check: output does not start with /nix/store")
	}
	return mainOutput, nil
}

type BuildOutput struct {
	Outputs map[string]string `json:"outputs"`
}
