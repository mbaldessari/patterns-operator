package values

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildValueFiles(t *testing.T) {
	t.Run("default files without prefix", func(t *testing.T) {
		files := BuildValueFiles("hub", "AWS", "4.12", "my-cluster", nil, "")
		expected := []string{
			"/values-global.yaml",
			"/values-hub.yaml",
			"/values-AWS.yaml",
			"/values-AWS-4.12.yaml",
			"/values-AWS-hub.yaml",
			"/values-4.12-hub.yaml",
			"/values-my-cluster.yaml",
		}
		assertStringSliceEqual(t, expected, files)
	})

	t.Run("default files with prefix", func(t *testing.T) {
		files := BuildValueFiles("hub", "AWS", "4.12", "my-cluster", nil, "$patternref")
		expected := []string{
			"$patternref/values-global.yaml",
			"$patternref/values-hub.yaml",
			"$patternref/values-AWS.yaml",
			"$patternref/values-AWS-4.12.yaml",
			"$patternref/values-AWS-hub.yaml",
			"$patternref/values-4.12-hub.yaml",
			"$patternref/values-my-cluster.yaml",
		}
		assertStringSliceEqual(t, expected, files)
	})

	t.Run("with extra value files", func(t *testing.T) {
		files := BuildValueFiles("hub", "AWS", "4.12", "my-cluster",
			[]string{"/test1.yaml", "test2.yaml"}, "")
		if len(files) != 9 {
			t.Fatalf("expected 9 files, got %d", len(files))
		}
		if files[7] != "/test1.yaml" {
			t.Errorf("expected /test1.yaml, got %s", files[7])
		}
		if files[8] != "/test2.yaml" {
			t.Errorf("expected /test2.yaml, got %s", files[8])
		}
	})
}

func TestMergeHelmValues(t *testing.T) {
	td := t.TempDir()

	t.Run("no files returns empty map", func(t *testing.T) {
		result, err := MergeHelmValues()
		assertNoError(t, err)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("missing files are skipped", func(t *testing.T) {
		result, err := MergeHelmValues("/nonexistent/file.yaml")
		assertNoError(t, err)
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("merges values from multiple files", func(t *testing.T) {
		file1 := filepath.Join(td, "values1.yaml")
		file2 := filepath.Join(td, "values2.yaml")
		writeFile(t, file1, "key1: value1\n")
		writeFile(t, file2, "key2: value2\n")

		result, err := MergeHelmValues(file1, file2)
		assertNoError(t, err)
		assertMapValue(t, result, "key1", "value1")
		assertMapValue(t, result, "key2", "value2")
	})

	t.Run("later files take precedence", func(t *testing.T) {
		file1 := filepath.Join(td, "values3.yaml")
		file2 := filepath.Join(td, "values4.yaml")
		writeFile(t, file1, "shared: from-file1\n")
		writeFile(t, file2, "shared: from-file2\n")

		result, err := MergeHelmValues(file1, file2)
		assertNoError(t, err)
		assertMapValue(t, result, "shared", "from-file2")
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		file := filepath.Join(td, "invalid.yaml")
		writeFile(t, file, "{{invalid yaml}}")

		_, err := MergeHelmValues(file)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})
}

func TestGetClusterGroupValue(t *testing.T) {
	t.Run("returns value when key exists", func(t *testing.T) {
		values := map[string]any{
			"clusterGroup": map[string]any{"name": "hub"},
		}
		result := GetClusterGroupValue("name", values)
		if result != "hub" {
			t.Errorf("expected hub, got %v", result)
		}
	})

	t.Run("returns nil when clusterGroup missing", func(t *testing.T) {
		values := map[string]any{"other": "value"}
		result := GetClusterGroupValue("name", values)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("returns nil when key missing in clusterGroup", func(t *testing.T) {
		values := map[string]any{
			"clusterGroup": map[string]any{"name": "hub"},
		}
		result := GetClusterGroupValue("nonexistent", values)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("returns nil when clusterGroup is not a map", func(t *testing.T) {
		values := map[string]any{"clusterGroup": "not-a-map"}
		result := GetClusterGroupValue("name", values)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
}

func TestHelmTpl(t *testing.T) {
	t.Run("templates with inline values", func(t *testing.T) {
		rendered, err := HelmTpl("Hello, {{ .Values.name }}!", nil, map[string]any{"name": "World"})
		assertNoError(t, err)
		if rendered != "Hello, World!" {
			t.Errorf("expected 'Hello, World!', got %q", rendered)
		}
	})

	t.Run("templates with value files", func(t *testing.T) {
		td := t.TempDir()
		f := filepath.Join(td, "values.yaml")
		writeFile(t, f, "name: FromFile\n")

		rendered, err := HelmTpl("Hello, {{ .Values.name }}!", []string{f}, map[string]any{})
		assertNoError(t, err)
		if rendered != "Hello, FromFile!" {
			t.Errorf("expected 'Hello, FromFile!', got %q", rendered)
		}
	})

	t.Run("skips missing value files gracefully", func(t *testing.T) {
		rendered, err := HelmTpl("Hello, {{ .Values.name }}!", []string{"/missing.yaml"}, map[string]any{"name": "World"})
		assertNoError(t, err)
		if rendered != "Hello, World!" {
			t.Errorf("expected 'Hello, World!', got %q", rendered)
		}
	})

	t.Run("static text", func(t *testing.T) {
		rendered, err := HelmTpl("static text", nil, map[string]any{})
		assertNoError(t, err)
		if rendered != "static text" {
			t.Errorf("expected 'static text', got %q", rendered)
		}
	})
}

func TestHelmTplStrict(t *testing.T) {
	t.Run("renders with all values provided", func(t *testing.T) {
		rendered, err := HelmTplStrict("platform: {{ .Values.global.clusterPlatform }}", nil,
			map[string]any{"global": map[string]any{"clusterPlatform": "AWS"}})
		assertNoError(t, err)
		if rendered != "platform: AWS" {
			t.Errorf("expected 'platform: AWS', got %q", rendered)
		}
	})

	t.Run("errors on missing value", func(t *testing.T) {
		_, err := HelmTplStrict("platform: {{ .Values.global.clusterPlatform }}", nil,
			map[string]any{"global": map[string]any{}})
		if err == nil {
			t.Fatal("expected error for missing value in strict mode")
		}
	})

	t.Run("static text renders without error", func(t *testing.T) {
		rendered, err := HelmTplStrict("no templates here", nil, map[string]any{})
		assertNoError(t, err)
		if rendered != "no templates here" {
			t.Errorf("expected 'no templates here', got %q", rendered)
		}
	})
}

func TestCountApplicationsAndSets(t *testing.T) {
	t.Run("nil returns 0,0", func(t *testing.T) {
		apps, appsets := CountApplicationsAndSets(nil)
		if apps != 0 || appsets != 0 {
			t.Errorf("expected 0,0, got %d,%d", apps, appsets)
		}
	})

	t.Run("apps only", func(t *testing.T) {
		input := map[string]any{
			"app1": map[string]any{"name": "app1"},
			"app2": map[string]any{"name": "app2"},
		}
		apps, appsets := CountApplicationsAndSets(input)
		if apps != 2 || appsets != 0 {
			t.Errorf("expected 2,0, got %d,%d", apps, appsets)
		}
	})

	t.Run("appsets only", func(t *testing.T) {
		input := map[string]any{
			"appset1": map[string]any{"generators": []any{"gen1"}},
			"appset2": map[string]any{"destinationServer": "server"},
		}
		apps, appsets := CountApplicationsAndSets(input)
		if apps != 0 || appsets != 2 {
			t.Errorf("expected 0,2, got %d,%d", apps, appsets)
		}
	})

	t.Run("mixed", func(t *testing.T) {
		input := map[string]any{
			"app1":    map[string]any{"name": "app1"},
			"appset1": map[string]any{"generators": []any{"gen1"}},
			"app2":    map[string]any{"chart": "chart1"},
		}
		apps, appsets := CountApplicationsAndSets(input)
		if apps != 2 || appsets != 1 {
			t.Errorf("expected 2,1, got %d,%d", apps, appsets)
		}
	})
}

func TestDotPathToNestedMap(t *testing.T) {
	t.Run("empty map", func(t *testing.T) {
		result := DotPathToNestedMap(map[string]string{})
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("flat keys", func(t *testing.T) {
		result := DotPathToNestedMap(map[string]string{
			"key1": "value1",
			"key2": "value2",
		})
		assertMapValue(t, result, "key1", "value1")
		assertMapValue(t, result, "key2", "value2")
	})

	t.Run("nested keys", func(t *testing.T) {
		result := DotPathToNestedMap(map[string]string{
			"global.clusterPlatform": "AWS",
			"global.clusterVersion":  "4.12",
		})
		global, ok := result["global"].(map[string]any)
		if !ok {
			t.Fatal("expected global to be a map")
		}
		if global["clusterPlatform"] != "AWS" {
			t.Errorf("expected AWS, got %v", global["clusterPlatform"])
		}
		if global["clusterVersion"] != "4.12" {
			t.Errorf("expected 4.12, got %v", global["clusterVersion"])
		}
	})

	t.Run("deeply nested keys", func(t *testing.T) {
		result := DotPathToNestedMap(map[string]string{
			"a.b.c": "deep",
		})
		a := result["a"].(map[string]any)
		b := a["b"].(map[string]any)
		if b["c"] != "deep" {
			t.Errorf("expected deep, got %v", b["c"])
		}
	})
}

func TestResolveSharedValueFiles(t *testing.T) {
	t.Run("nonexistent path returns error", func(t *testing.T) {
		input := ResolveInput{LocalCheckoutPath: "/nonexistent/path"}
		_, err := ResolveSharedValueFiles(&input, "")
		if err == nil {
			t.Fatal("expected error for nonexistent path")
		}
	})

	t.Run("no sharedValueFiles returns nil", func(t *testing.T) {
		td := t.TempDir()
		writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")
		writeFile(t, filepath.Join(td, "values-hub.yaml"), "clusterGroup:\n  name: hub\n")

		input := ResolveInput{
			LocalCheckoutPath: td,
			ClusterGroupName:  "hub",
		}
		result, err := ResolveSharedValueFiles(&input, "")
		assertNoError(t, err)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("resolves sharedValueFiles with templating", func(t *testing.T) {
		td := t.TempDir()
		writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  clusterPlatform: AWS\n")
		writeFile(t, filepath.Join(td, "values-hub.yaml"),
			"clusterGroup:\n  name: hub\n  sharedValueFiles:\n    - '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'\n")

		input := ResolveInput{
			LocalCheckoutPath: td,
			ClusterGroupName:  "hub",
			TemplateParameters: map[string]string{
				"global.clusterPlatform": "AWS",
			},
		}
		result, err := ResolveSharedValueFiles(&input, "")
		assertNoError(t, err)
		if len(result) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(result), result)
		}
		expected := "/overrides/values-AWS.yaml"
		if result[0] != expected {
			t.Errorf("expected %q, got %q", expected, result[0])
		}
	})

	t.Run("resolves with prefix", func(t *testing.T) {
		td := t.TempDir()
		writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  clusterPlatform: Azure\n")
		writeFile(t, filepath.Join(td, "values-hub.yaml"),
			"clusterGroup:\n  name: hub\n  sharedValueFiles:\n    - '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'\n")

		input := ResolveInput{
			LocalCheckoutPath: td,
			ClusterGroupName:  "hub",
			TemplateParameters: map[string]string{
				"global.clusterPlatform": "Azure",
			},
		}
		result, err := ResolveSharedValueFiles(&input, "$patternref")
		assertNoError(t, err)
		if len(result) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(result), result)
		}
		expected := "$patternref/overrides/values-Azure.yaml"
		if result[0] != expected {
			t.Errorf("expected %q, got %q", expected, result[0])
		}
	})

	t.Run("uses TemplateValues over TemplateParameters", func(t *testing.T) {
		td := t.TempDir()
		// No clusterPlatform in values-global.yaml so TemplateValues wins
		writeFile(t, filepath.Join(td, "values-global.yaml"), "global:\n  pattern: test\n")
		writeFile(t, filepath.Join(td, "values-hub.yaml"),
			"clusterGroup:\n  name: hub\n  sharedValueFiles:\n    - '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'\n")

		input := ResolveInput{
			LocalCheckoutPath: td,
			ClusterGroupName:  "hub",
			TemplateParameters: map[string]string{
				"global.clusterPlatform": "WRONG",
			},
			TemplateValues: map[string]any{
				"global": map[string]any{"clusterPlatform": "GCP"},
			},
		}
		result, err := ResolveSharedValueFiles(&input, "")
		assertNoError(t, err)
		if len(result) != 1 {
			t.Fatalf("expected 1 result, got %d", len(result))
		}
		expected := "/overrides/values-GCP.yaml"
		if result[0] != expected {
			t.Errorf("expected %q, got %q", expected, result[0])
		}
	})
}

func TestFindCharts(t *testing.T) {
	td := t.TempDir()

	t.Run("finds charts in subdirectories", func(t *testing.T) {
		chart1 := filepath.Join(td, "charts", "app1")
		chart2 := filepath.Join(td, "charts", "app2")
		os.MkdirAll(chart1, 0755)
		os.MkdirAll(chart2, 0755)
		writeFile(t, filepath.Join(chart1, "Chart.yaml"), "apiVersion: v2\nname: app1\nversion: 0.1.0\n")
		writeFile(t, filepath.Join(chart2, "Chart.yaml"), "apiVersion: v2\nname: app2\nversion: 0.1.0\n")

		charts, err := FindCharts(td)
		assertNoError(t, err)
		if len(charts) != 2 {
			t.Fatalf("expected 2 charts, got %d: %v", len(charts), charts)
		}
	})

	t.Run("skips vendor directory", func(t *testing.T) {
		vendorChart := filepath.Join(td, "vendor", "somechart")
		os.MkdirAll(vendorChart, 0755)
		writeFile(t, filepath.Join(vendorChart, "Chart.yaml"), "apiVersion: v2\nname: vendor\nversion: 0.1.0\n")

		charts, err := FindCharts(td)
		assertNoError(t, err)
		for _, c := range charts {
			if filepath.Base(c) == "somechart" {
				t.Errorf("should skip vendor directory, found %s", c)
			}
		}
	})

	t.Run("empty directory returns nil", func(t *testing.T) {
		empty := t.TempDir()
		charts, err := FindCharts(empty)
		assertNoError(t, err)
		if len(charts) != 0 {
			t.Errorf("expected no charts, got %v", charts)
		}
	})
}

func TestRenderChart(t *testing.T) {
	t.Run("renders chart templates with values", func(t *testing.T) {
		td := t.TempDir()
		os.MkdirAll(filepath.Join(td, "templates"), 0755)
		writeFile(t, filepath.Join(td, "Chart.yaml"), "apiVersion: v2\nname: test\nversion: 0.1.0\n")
		writeFile(t, filepath.Join(td, "values.yaml"), "name: default\n")
		writeFile(t, filepath.Join(td, "templates", "cm.yaml"),
			"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Values.name }}\n")

		rendered, err := RenderChart(td, map[string]any{"name": "override"})
		assertNoError(t, err)
		found := false
		for _, content := range rendered {
			if content != "" {
				found = true
				if !containsStr(content, "name: override") {
					t.Errorf("expected rendered name 'override', got:\n%s", content)
				}
			}
		}
		if !found {
			t.Error("no rendered templates found")
		}
	})

	t.Run("errors on missing value in strict mode", func(t *testing.T) {
		td := t.TempDir()
		os.MkdirAll(filepath.Join(td, "templates"), 0755)
		writeFile(t, filepath.Join(td, "Chart.yaml"), "apiVersion: v2\nname: test\nversion: 0.1.0\n")
		writeFile(t, filepath.Join(td, "values.yaml"), "")
		writeFile(t, filepath.Join(td, "templates", "cm.yaml"),
			"value: {{ .Values.missing.key }}\n")

		_, err := RenderChart(td, map[string]any{})
		if err == nil {
			t.Fatal("expected error for missing value in strict mode")
		}
	})

	t.Run("errors on invalid chart path", func(t *testing.T) {
		_, err := RenderChart("/nonexistent/chart", map[string]any{})
		if err == nil {
			t.Fatal("expected error for nonexistent chart")
		}
	})
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}

// Test helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertMapValue(t *testing.T, m map[string]any, key string, expected any) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Fatalf("key %q not found in map", key)
	}
	if val != expected {
		t.Errorf("expected %v for key %q, got %v", expected, key, val)
	}
}

func assertStringSliceEqual(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("length mismatch: expected %d, got %d\nexpected: %v\nactual:   %v", len(expected), len(actual), expected, actual)
	}
	for i := range expected {
		if expected[i] != actual[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], actual[i])
		}
	}
}
