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
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: hub\n")
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

func TestRunClusterGroupFromGlobal(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: group-one\n")
	writeFile(t, filepath.Join(td, "values-group-one.yaml"), "clusterGroup:\n  name: group-one\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--patterndir", td}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "values-group-one.yaml") {
		t.Errorf("expected group-one value file from auto-detected cluster group:\n%s", out)
	}
	if strings.Contains(out, "values-hub.yaml") {
		t.Errorf("should not contain hub when auto-detected group is group-one:\n%s", out)
	}
}

func TestRunMissingClusterGroupName(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--patterndir", td}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "could not read main.clusterGroupName") {
		t.Errorf("expected clusterGroupName error on stderr, got: %s", stderr.String())
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
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: hub\n")
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

func TestRenderValueFiles(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: hub\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"),
		"clusterGroup:\n  name: hub\n  sharedValueFiles:\n    - '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-platform", "AWS",
		"-render-value-files",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "# "+filepath.Join(td, "values-global.yaml")) {
		t.Errorf("missing rendered header for values-global.yaml:\n%s", out)
	}
	if !strings.Contains(out, "# "+filepath.Join(td, "values-hub.yaml")) {
		t.Errorf("missing rendered header for values-hub.yaml:\n%s", out)
	}
	if !strings.Contains(out, "'/overrides/values-AWS.yaml'") {
		t.Errorf("expected template to be rendered with AWS:\n%s", out)
	}
}

func TestRenderValueFilesSkipsMissing(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: hub\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-platform", "AWS",
		"-render-value-files",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "# "+filepath.Join(td, "values-global.yaml")) {
		t.Errorf("missing rendered header for values-global.yaml:\n%s", out)
	}
	if strings.Contains(out, "# "+filepath.Join(td, "values-hub.yaml")) {
		t.Errorf("should not render missing values-hub.yaml:\n%s", out)
	}
}

func TestRenderValueFilesErrorOnMissingVar(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: hub\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"),
		"platform: '{{ $.Values.global.clusterPlatform }}'\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-group", "hub",
		"-render-value-files",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing variable, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Error rendering") {
		t.Errorf("expected rendering error on stderr, got: %s", stderr.String())
	}
}

func TestRenderValueFilesMultiple(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\nmain:\n  clusterGroupName: hub\n")
	writeFile(t, filepath.Join(td, "values-hub.yaml"), "clusterGroup:\n  name: hub\n")
	writeFile(t, filepath.Join(td, "values-AWS.yaml"), "aws:\n  region: us-east-1\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-platform", "AWS",
		"-render-value-files",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "# "+filepath.Join(td, "values-global.yaml")) {
		t.Errorf("missing values-global.yaml rendered output:\n%s", out)
	}
	if !strings.Contains(out, "# "+filepath.Join(td, "values-hub.yaml")) {
		t.Errorf("missing values-hub.yaml rendered output:\n%s", out)
	}
	if !strings.Contains(out, "# "+filepath.Join(td, "values-AWS.yaml")) {
		t.Errorf("missing values-AWS.yaml rendered output:\n%s", out)
	}
	if !strings.Contains(out, "region: us-east-1") {
		t.Errorf("expected AWS content in rendered output:\n%s", out)
	}
}

func TestRenderValueFilesStripsComments(t *testing.T) {
	td := t.TempDir()
	writeFile(t, filepath.Join(td, "values-global.yaml"),
		"global:\n  pattern: test\n# this is a comment\n  # indented comment\nmain:\n  clusterGroupName: hub\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--patterndir", td,
		"--cluster-platform", "AWS",
		"-render-value-files",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	out := stdout.String()
	if strings.Contains(out, "this is a comment") {
		t.Errorf("comments should be stripped from rendered output:\n%s", out)
	}
	if strings.Contains(out, "indented comment") {
		t.Errorf("indented comments should be stripped from rendered output:\n%s", out)
	}
	if !strings.Contains(out, "pattern: test") {
		t.Errorf("non-comment content should still be present:\n%s", out)
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
