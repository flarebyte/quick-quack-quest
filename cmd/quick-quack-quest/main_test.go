package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRunSuccessAndFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if code := run([]string{"version", "--format", "json"}, &stderr); code != 0 {
		t.Fatalf("expected success exit code, got %d stderr=%s", code, stderr.String())
	}

	stderr.Reset()
	if code := run([]string{"query", "run"}, &stderr); code != 1 {
		t.Fatalf("expected failure exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "accepts 1 arg") {
		t.Fatalf("expected argument validation error in stderr, got: %s", stderr.String())
	}
}

func TestMainUsesExitCodeFromRun(t *testing.T) {
	origExit := osExit
	origArgs := os.Args
	defer func() {
		osExit = origExit
		os.Args = origArgs
	}()

	captured := -1
	osExit = func(code int) { captured = code }
	os.Args = []string{"quack", "version", "--format", "json"}

	main()
	if captured != 0 {
		t.Fatalf("expected main to exit with 0, got %d", captured)
	}
}
