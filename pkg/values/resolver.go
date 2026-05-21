package values

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// ResolveInput captures everything needed to resolve shared value files
// without any K8s or ArgoCD type dependencies.
type ResolveInput struct {
	// LocalCheckoutPath is the filesystem path to the checked-out git repo
	LocalCheckoutPath string

	// ClusterGroupName e.g. "hub", "group-one"
	ClusterGroupName string

	// ClusterPlatform e.g. "AWS", "Azure"
	ClusterPlatform string

	// ClusterVersion e.g. "4.12"
	ClusterVersion string

	// ClusterName e.g. "my-hub-cluster"
	ClusterName string

	// ExtraValueFiles are additional value file paths (relative to repo root)
	ExtraValueFiles []string

	// TemplateParameters are flat dot-separated key=value pairs used when
	// templating sharedValueFiles entries (e.g. "global.clusterPlatform": "AWS").
	// These get expanded into a nested map for Helm templating.
	// Ignored if TemplateValues is set.
	TemplateParameters map[string]string

	// TemplateValues is a pre-built nested map used for Helm templating.
	// If set, takes precedence over TemplateParameters.
	TemplateValues map[string]any
}

// BuildValueFiles constructs the ordered list of value file paths for a
// given cluster configuration.
func BuildValueFiles(clusterGroupName, clusterPlatform, clusterVersion, clusterName string, extraValueFiles []string, prefix string) []string {
	files := []string{
		fmt.Sprintf("%s/values-global.yaml", prefix),
		fmt.Sprintf("%s/values-%s.yaml", prefix, clusterGroupName),
		fmt.Sprintf("%s/values-%s.yaml", prefix, clusterPlatform),
		fmt.Sprintf("%s/values-%s-%s.yaml", prefix, clusterPlatform, clusterVersion),
		fmt.Sprintf("%s/values-%s-%s.yaml", prefix, clusterPlatform, clusterGroupName),
		fmt.Sprintf("%s/values-%s-%s.yaml", prefix, clusterVersion, clusterGroupName),
		fmt.Sprintf("%s/values-%s.yaml", prefix, clusterName),
	}

	for _, extra := range extraValueFiles {
		extraValueFile := fmt.Sprintf("%s/%s", prefix, strings.TrimPrefix(extra, "/"))
		log.Printf("Values file %q added", extraValueFile)
		files = append(files, extraValueFile)
	}
	return files
}

// MergeHelmValues reads and merges multiple YAML value files using Helm's
// CoalesceTables. Missing files are silently skipped.
func MergeHelmValues(files ...string) (map[string]any, error) {
	mergedValues := make(map[string]any)

	for _, file := range files {
		var values map[string]any
		values, err := chartutil.ReadValuesFile(file)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Skip this file as it does not exist
				continue
			}
			// For all other errors, return the error
			return nil, err
		}
		// Contrary to intuition the dst argument (values) takes precedence
		mergedValues = chartutil.CoalesceTables(values, mergedValues)
	}

	return mergedValues, nil
}

// GetClusterGroupValue extracts a key from the "clusterGroup" section of
// merged Helm values. Returns nil if the clusterGroup section is missing,
// is not a map, or does not contain the key.
func GetClusterGroupValue(key string, values map[string]any) any {
	clusterGroup, hasClusterGroup := values["clusterGroup"]
	if !hasClusterGroup {
		return nil
	}

	clusterGroupMap, ok := clusterGroup.(map[string]any)
	if !ok {
		return nil
	}
	v, hasKey := clusterGroupMap[key]
	if hasKey {
		return v
	}
	return nil
}

// HelmTpl templates a string using Helm's engine with the given value
// files and additional values map.
func HelmTpl(templateString string, valueFiles []string, values map[string]any) (string, error) {
	// Create a fake chart with the template.
	fakeChart := &chart.Chart{
		Metadata: &chart.Metadata{
			APIVersion: "v2",
			Name:       "fake",
			Version:    "0.1.0",
		},
		Templates: []*chart.File{
			{
				Name: "templates/template.tpl",
				Data: []byte(templateString),
			},
		},
	}

	// Load and merge values from the specified value files. Note that the ordering is a bit
	// unexpected. The first values added are the more specific ones that will win
	mergedValues := make(map[string]any)
	// Contrary to intuition the dst argument (values) takes precedence
	mergedValues = chartutil.CoalesceTables(values, mergedValues)
	for _, fileName := range valueFiles {
		fname := filepath.Clean(fileName)
		// If the file does not exist we simply skip it
		if _, err := os.Stat(fname); os.IsNotExist(err) {
			continue
		}
		fileValues, err := chartutil.ReadValuesFile(fname)
		if err != nil {
			return "", fmt.Errorf("error reading values file %s: %w", fileName, err)
		}
		// Contrary to intuition the dst argument (values) takes precedence
		mergedValues = chartutil.CoalesceTables(fileValues, mergedValues)
	}

	// Merge with the additional values provided.
	mergedValues = chartutil.CoalesceTables(mergedValues, values)

	// Render the template.
	options := chartutil.ReleaseOptions{
		Name:      "fake-release",
		Namespace: "default",
		IsInstall: true,
		IsUpgrade: false,
	}
	valuesToRender, err := chartutil.ToRenderValues(fakeChart, mergedValues, options, chartutil.DefaultCapabilities)
	if err != nil {
		return "", fmt.Errorf("error preparing render values: %w", err)
	}

	renderedTemplates, err := engine.Render(fakeChart, valuesToRender)
	if err != nil {
		return "", fmt.Errorf("error rendering template: %w", err)
	}

	// Extract the rendered template.
	rendered, ok := renderedTemplates["fake/templates/template.tpl"]
	if !ok {
		return "", fmt.Errorf("rendered template not found")
	}

	return rendered, nil
}

// CountApplicationsAndSets inspects an "applications" map from Helm values
// and returns counts of regular applications vs ApplicationSets.
func CountApplicationsAndSets(a any) (appCount, appSetsCount int) {
	applicationCount := 0
	applicationSetsCount := 0
	applicationSetsKeys := []string{"generators", "generatorFile", "useGeneratorValues", "destinationServer", "destinationNamespace"}

	m, ok := a.(map[string]any)
	if !ok {
		return 0, 0
	}
	for _, v := range m {
		foundApplicationSet := false
		subMap, ok := v.(map[string]any)
		if !ok {
			// If it's not a map, skip it
			continue
		}
		// ApplicationSets have one of these subkeys in the application
		for _, key := range applicationSetsKeys {
			if _, exists := subMap[key]; exists {
				foundApplicationSet = true
				break
			}
		}
		if foundApplicationSet {
			applicationSetsCount++
		} else {
			applicationCount++
		}
	}
	return applicationCount, applicationSetsCount
}

// DotPathToNestedMap converts a flat map of dot-separated keys to a nested
// map[string]any suitable for Helm templating.
// e.g. {"global.clusterPlatform": "AWS"} -> {"global": {"clusterPlatform": "AWS"}}
func DotPathToNestedMap(params map[string]string) map[string]any {
	result := make(map[string]any)

	for name, value := range params {
		keys := strings.Split(name, ".")
		lastKeyIndex := len(keys) - 1

		currentMap := result
		for i, key := range keys {
			if i == lastKeyIndex {
				currentMap[key] = value
			} else {
				if _, ok := currentMap[key]; !ok {
					currentMap[key] = make(map[string]any)
				}
				currentMap = currentMap[key].(map[string]any)
			}
		}
	}
	return result
}

// ResolveSharedValueFiles resolves the clusterGroup.sharedValueFiles from a
// checked-out git repo.
//  1. Builds the value file list from the input configuration
//  2. Merges them in order
//  3. Extracts clusterGroup.sharedValueFiles
//  4. Templates each entry using Helm (e.g. '/overrides/values-{{ $.Values.global.clusterPlatform }}.yaml'
//     becomes '/overrides/values-AWS.yaml')
//  5. Returns the resolved file paths with the given prefix prepended
func ResolveSharedValueFiles(input *ResolveInput, prefix string) ([]string, error) {
	gitDir := input.LocalCheckoutPath
	if _, err := os.Stat(gitDir); err != nil {
		return nil, fmt.Errorf("%s path does not exist", gitDir)
	}

	valueFiles := BuildValueFiles(input.ClusterGroupName, input.ClusterPlatform,
		input.ClusterVersion, input.ClusterName, input.ExtraValueFiles, gitDir)

	helmValues, err := MergeHelmValues(valueFiles...)
	if err != nil {
		return nil, fmt.Errorf("could not fetch value files: %s", err)
	}
	sharedValueFiles := GetClusterGroupValue("sharedValueFiles", helmValues)
	if sharedValueFiles == nil {
		return nil, nil
	}

	// Check if s is of type []interface{}
	val, ok := sharedValueFiles.([]any)
	if !ok {
		return nil, fmt.Errorf("could not make a list out of sharedValueFiles: %v", sharedValueFiles)
	}

	// Build template values map
	var valueMap map[string]any
	if input.TemplateValues != nil {
		valueMap = input.TemplateValues
	} else if input.TemplateParameters != nil {
		valueMap = DotPathToNestedMap(input.TemplateParameters)
	} else {
		valueMap = make(map[string]any)
	}

	// Convert each element of slice to a string
	var result []string
	for i, v := range val {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("type assertion failed at index %d: Not a string", i)
		}
		templatedString, err := HelmTpl(str, valueFiles, valueMap)

		// we only log an error, but try to keep going
		if err != nil {
			log.Printf("Failed to render templated string %s: %v", str, err)
			continue
		}
		if strings.HasPrefix(templatedString, "/") {
			result = append(result, fmt.Sprintf("%s%s", prefix, templatedString))
		} else {
			result = append(result, fmt.Sprintf("%s/%s", prefix, templatedString))
		}
	}

	return result, nil
}
