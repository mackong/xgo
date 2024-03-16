package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"strings"

	"github.com/xhd2015/xgo/support/filecopy"
	"github.com/xhd2015/xgo/support/goinfo"
)

func getTempFile(pattern string) (string, error) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), pattern)
	if err != nil {
		return "", err
	}

	return filepath.Join(tmpDir, pattern), nil
}

type options struct {
	run    bool
	noTrim bool
	env    []string

	noPipeStderr bool
}

func xgoBuild(args []string, opts *options) (string, error) {
	var xgoCmd string = "build"
	if opts != nil {
		if opts.run {
			xgoCmd = "run"
		}
	}
	buildArgs := append([]string{
		"run", "../cmd/xgo",
		xgoCmd,
		"--xgo-src",
		"../",
		"--sync-with-link",
	}, args...)
	cmd := exec.Command("go", buildArgs...)
	if opts == nil || !opts.noPipeStderr {
		cmd.Stderr = os.Stderr
	}
	if opts != nil && len(opts.env) > 0 {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, opts.env...)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	outStr := string(output)
	if opts == nil || !opts.noTrim {
		outStr = strings.TrimSuffix(outStr, "\n")
	}
	return outStr, nil
}

// return clean up func
type buildRuntimeOpts struct {
	xgoBuildArgs []string
	xgoBuildEnv  []string
	runEnv       []string

	debug bool
}

func buildWithRuntimeAndOutput(dir string, opts buildRuntimeOpts) (string, error) {
	tmpFile, err := getTempFile("test")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpFile)

	// func_list depends on xgo/runtime, but xgo/runtime is
	// a separate module, so we need to merge them
	// together first
	tmpDir, funcListDir, err := tmpMergeRuntimeAndTest(dir)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	xgoBuildArgs := []string{
		"-o", tmpFile,
		// "-a",
		"--project-dir", funcListDir,
	}
	if opts.debug {
		xgoBuildArgs = append(xgoBuildArgs, "-gcflags=all=-N -l")
	}
	xgoBuildArgs = append(xgoBuildArgs, opts.xgoBuildArgs...)
	xgoBuildArgs = append(xgoBuildArgs, ".")
	_, err = xgoBuild(xgoBuildArgs, &options{
		env: opts.xgoBuildEnv,
	})
	if err != nil {
		return "", err
	}
	if opts.debug {
		fmt.Println(tmpFile)
		time.Sleep(10 * time.Minute)
	}
	runCmd := exec.Command(tmpFile)
	runCmd.Env = os.Environ()
	runCmd.Env = append(runCmd.Env, opts.runEnv...)
	output, err := runCmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func tmpMergeRuntimeAndTest(testDir string) (rootDir string, subDir string, err error) {
	return linkRuntimeAndTest(testDir, false)
}

func tmpRuntimeModeAndTest(testDir string) (rootDir string, subDir string, err error) {
	return linkRuntimeAndTest(testDir, true)
}

func linkRuntimeAndTest(testDir string, goModOnly bool) (rootDir string, subDir string, err error) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "test")
	if err != nil {
		return "", "", err
	}

	// copy runtime to a tmp directory, and
	// test under there
	if goModOnly {
		err = filecopy.LinkFile("../runtime/go.mod", filepath.Join(tmpDir, "go.mod"))
	} else {
		err = filecopy.LinkFiles("../runtime", tmpDir)
	}
	if err != nil {
		return "", "", err
	}

	subDir, err = os.MkdirTemp(tmpDir, filepath.Base(testDir))
	if err != nil {
		return "", "", err
	}

	err = filecopy.LinkFiles(testDir, subDir)
	if err != nil {
		return "", "", err
	}
	return tmpDir, subDir, nil
}

func fatalExecErr(t *testing.T, err error) {
	if err, ok := err.(*exec.ExitError); ok {
		t.Fatalf("%v", string(err.Stderr))
	}
	t.Fatalf("%v", err)
}

func getErrMsg(err error) string {
	if err, ok := err.(*exec.ExitError); ok {
		return string(err.Stderr)
	}
	return err.Error()
}

func buildAndRunOutput(program string) (output string, err error) {
	return buildAndRunOutputArgs([]string{program}, buildAndOutputOptions{})
}

type buildAndOutputOptions struct {
	build func(args []string) error
}

func buildAndRunOutputArgs(args []string, opts buildAndOutputOptions) (output string, err error) {
	testBin, err := getTempFile("test")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(testBin)
	buildArgs := []string{"-o", testBin}
	buildArgs = append(buildArgs, args...)
	if opts.build != nil {
		err = opts.build(buildArgs)
	} else {
		_, err = xgoBuild(buildArgs, nil)
	}
	if err != nil {
		return "", err
	}
	out, err := exec.Command(testBin).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func getGoVersion() (*goinfo.GoVersion, error) {
	goVersionStr, err := goinfo.GetGoVersionOutput("go")
	if err != nil {
		return nil, err
	}
	return goinfo.ParseGoVersion(goVersionStr)
}
