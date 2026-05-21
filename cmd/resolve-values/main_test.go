package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestRunDefaults(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"), "clusterGroup:\n  name: hub\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--patterndir", td}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Base value files:") {
		t.Error("missing 'Base value files:' header")
	}
	if !strings.Contains(out, "[exists ] "+filepath.Join(td, "values-global.yaml")) {
		t.Errorf("values-global.yaml should be marked as exists:\n%s", out)
	}
	if !strings.Contains(out, "[exists ] "+filepath.Join(td, "values-hub.yaml")) {
		t.Errorf("values-hub.yaml should be marked as exists:\n%s", out)
	}
	if !strings.Contains(out, "[missing]") {
		t.Error("expected some files marked as missing")
	}
	if !strings.Contains(out, "No shared value files defined.") {
		t.Error("expected 'No shared value files defined.' when none configured")
	}
}

func TestRunWithClusterParams(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"), "clusterGroup:\n  name: hub\n")
	writeFile(t, filepath.Join(td, "values-AWS.yaml"), "aws:\n  region: us-east-1\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-group", "hub",
		"--cluster-platform", "AWS",
		"--cluster-version", "4.16",
		"--cluster-name", "mycluster",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "[exists ] "+filepath.Join(td, "values-AWS.yaml")) {
		t.Errorf("values-AWS.yaml should be marked as exists:\n%s", out)
	}
	if !strings.Contains(out, "[missing] "+filepath.Join(td, "values-AWS-4.16.yaml")) {
		t.Errorf("values-AWS-4.16.yaml should be marked as missing:\n%s", out)
	}
	if !strings.Contains(out, "[missing] "+filepath.Join(td, "values-mycluster.yaml")) {
		t.Errorf("values-mycluster.yaml should be marked as missing:\n%s", out)
	}
}

func TestRunWithSharedValueFiles(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  clusterPlatform: AWS\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"),
		"clusterGroup:\n  name: hub\n  sharedValueFiles:\n    - '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-group", "hub",
		"--cluster-platform", "AWS",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Shared value files (from clusterGroup.sharedValueFiles):") {
		t.Errorf("missing shared value files header:\n%s", out)
	}
	if !strings.Contains(out, "/overrides/values-AWS.yaml") {
		t.Errorf("expected resolved shared value file path:\n%s", out)
	}
}

func TestRunWithExtraValueFiles(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")
	writeFile(t, filepath.Join(td, "extra.yaml"), "extra: value\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--extra-value-files", "extra.yaml,another.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "[exists ] "+filepath.Join(td, "extra.yaml")) {
		t.Errorf("extra.yaml should be marked as exists:\n%s", out)
	}
	if !strings.Contains(out, "[missing] "+filepath.Join(td, "another.yaml")) {
		t.Errorf("another.yaml should be marked as missing:\n%s", out)
	}
}

func TestRunMissingPatterndir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--cluster-group", "hub"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--patterndir is required") {
		t.Errorf("expected gitdir required error on stderr, got: %s", stderr.String())
	}
}

func TestRunNonexistentPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--patterndir", "/nonexistent/path"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Path does not exist") {
		t.Errorf("expected path error on stderr, got: %s", stderr.String())
	}
}

func TestRunInvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--invalid-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRunMultipleSharedValueFiles(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  clusterPlatform: Azure\n  clusterVersion: \"4.16\"\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"),
		"clusterGroup:\n  name: hub\n  sharedValueFiles:\n    - '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'\n    - '/overrides/values-{{ $.Values.global.clusterVersion }}.yaml'\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-group", "hub",
		"--cluster-platform", "Azure",
		"--cluster-version", "4.16",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "/overrides/values-Azure.yaml") {
		t.Errorf("expected Azure shared value file:\n%s", out)
	}
	if !strings.Contains(out, "/overrides/values-4.16.yaml") {
		t.Errorf("expected 4.16 shared value file:\n%s", out)
	}
}

func TestRunNonHubClusterGroup(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")
	writeFile(t, filepath.Join(td, "values-group-one.yaml"), "clusterGroup:\n  name: group-one\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-group", "group-one",
		"--cluster-platform", "AWS",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "values-group-one.yaml") {
		t.Errorf("expected group-one value file:\n%s", out)
	}
	if !strings.Contains(out, "values-AWS-group-one.yaml") {
		t.Errorf("expected platform-group combined value file:\n%s", out)
	}
	if strings.Contains(out, "values-hub.yaml") {
		t.Errorf("should not contain hub value file:\n%s", out)
	}
}
