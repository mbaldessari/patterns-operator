package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hybrid-cloud-patterns/patterns-operator/pkg/values"
	"helm.sh/helm/v3/pkg/chartutil"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("resolve-values", flag.ContinueOnError)
	fs.SetOutput(stderr)

	patternDir := fs.String("patterndir", "", "Path to the pattern's git repo checkout (required)")
	clusterGroup := fs.String("cluster-group", "", "Cluster group name (default: read from values-global.yaml)")
	clusterPlatform := fs.String("cluster-platform", "", "Cluster platform (e.g. AWS, Azure)")
	clusterVersion := fs.String("cluster-version", "", "Cluster version (e.g. 4.12)")
	clusterName := fs.String("cluster-name", "", "Cluster name")
	hubClusterDomain := fs.String("hub-cluster-domain", "", "Hub cluster application domain (e.g. apps.hub.example.com)")
	localClusterDomain := fs.String("local-cluster-domain", "", "Local cluster application domain")
	clusterID := fs.String("cluster-id", "", "Cluster ID")
	extraValueFiles := fs.String("extra-value-files", "", "Comma-separated extra value file paths")
	renderValueFiles := fs.Bool("render-value-files", false, "Render and print existing value files with cluster parameters substituted")
	renderHelm := fs.Bool("render-helm", false, "Find and render all Helm chart templates with merged values")
	withManagedCluster := fs.String("with-managedcluster", "", "Name of a managedClusterGroups entry to also render (requires -render-helm)")
	spokeClusterName := fs.String("spoke-cluster-name", "", "Spoke cluster name (required with -with-managedcluster)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *renderValueFiles && *renderHelm {
		fmt.Fprintln(stderr, "Error: -render-value-files and -render-helm are mutually exclusive")
		return 1
	}

	if *withManagedCluster != "" && !*renderHelm {
		fmt.Fprintln(stderr, "Error: -with-managedcluster requires -render-helm")
		return 1
	}

	if *withManagedCluster != "" && *spokeClusterName == "" {
		fmt.Fprintln(stderr, "Error: -spoke-cluster-name is required when -with-managedcluster is set")
		return 1
	}

	if *patternDir == "" {
		fmt.Fprintln(stderr, "Error: --patterndir is required")
		fs.Usage()
		return 1
	}

	absPath, err := filepath.Abs(*patternDir)
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving path: %v\n", err)
		return 1
	}

	if _, err := os.Stat(absPath); err != nil {
		fmt.Fprintf(stderr, "Path does not exist: %s\n", absPath)
		return 1
	}

	if *clusterGroup == "" {
		globalFile := filepath.Join(absPath, "values-global.yaml")
		v, err := values.MergeHelmValues(globalFile)
		if err != nil {
			fmt.Fprintf(stderr, "Error reading %s: %v\n", globalFile, err)
			return 1
		}
		mainSection, _ := v["main"].(map[string]any)
		if name, ok := mainSection["clusterGroupName"].(string); ok && name != "" {
			*clusterGroup = name
		} else {
			fmt.Fprintln(stderr, "Error: could not read main.clusterGroupName from values-global.yaml; use --cluster-group")
			return 1
		}
	}

	var extras []string
	if *extraValueFiles != "" {
		extras = strings.Split(*extraValueFiles, ",")
	}

	baseFiles := values.BuildValueFiles(*clusterGroup, *clusterPlatform,
		*clusterVersion, *clusterName, extras, absPath)

	fmt.Fprintln(stdout, "Base value files:")
	for _, f := range baseFiles {
		status := "exists "
		if _, err := os.Stat(f); os.IsNotExist(err) {
			status = "missing"
		}
		fmt.Fprintf(stdout, "  [%s] %s\n", status, f)
	}

	templateParams := make(map[string]string)
	if *clusterPlatform != "" {
		templateParams["global.clusterPlatform"] = *clusterPlatform
	}
	if *clusterVersion != "" {
		templateParams["global.clusterVersion"] = *clusterVersion
	}
	if *clusterName != "" {
		templateParams["global.clusterDomain"] = *clusterName
	}

	input := values.ResolveInput{
		LocalCheckoutPath:  absPath,
		ClusterGroupName:   *clusterGroup,
		ClusterPlatform:    *clusterPlatform,
		ClusterVersion:     *clusterVersion,
		ClusterName:        *clusterName,
		ExtraValueFiles:    extras,
		TemplateParameters: templateParams,
	}

	sharedFiles, err := values.ResolveSharedValueFiles(&input, absPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving shared value files: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout)
	if len(sharedFiles) == 0 {
		fmt.Fprintln(stdout, "No shared value files defined.")
	} else {
		fmt.Fprintln(stdout, "Shared value files (from clusterGroup.sharedValueFiles):")
		for _, f := range sharedFiles {
			fmt.Fprintf(stdout, "  %s\n", f)
		}
	}

	if *renderValueFiles {
		renderVals := buildRenderValues(*clusterGroup, *clusterPlatform, *clusterVersion,
			*clusterName, *hubClusterDomain, *localClusterDomain, *clusterID)

		allFiles := append(baseFiles, sharedFiles...)
		for _, f := range allFiles {
			if _, err := os.Stat(f); os.IsNotExist(err) {
				continue
			}
			content, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(stderr, "Error reading %s: %v\n", f, err)
				return 1
			}
			rendered, err := values.HelmTplStrict(string(content), nil, renderVals)
			if err != nil {
				fmt.Fprintf(stderr, "Error rendering %s: %v\n", f, err)
				return 1
			}
			fmt.Fprintf(stdout, "\n# %s\n", f)
			printStrippingComments(stdout, rendered)
		}
	}

	if *renderHelm {
		var existingFiles []string
		for _, f := range append(baseFiles, sharedFiles...) {
			if _, err := os.Stat(f); !os.IsNotExist(err) {
				existingFiles = append(existingFiles, f)
			}
		}

		mergedValues, err := values.MergeHelmValues(existingFiles...)
		if err != nil {
			fmt.Fprintf(stderr, "Error merging value files: %v\n", err)
			return 1
		}
		cliValues := buildRenderValues(*clusterGroup, *clusterPlatform, *clusterVersion,
			*clusterName, *hubClusterDomain, *localClusterDomain, *clusterID)
		mergedValues = chartutil.CoalesceTables(cliValues, mergedValues)

		var spokeGroupName string
		var spokeGroupKey string
		if *withManagedCluster != "" {
			mcGroups := values.GetClusterGroupValue("managedClusterGroups", mergedValues)
			if mcGroups == nil {
				fmt.Fprintf(stderr, "Error: no managedClusterGroups found in hub values\n")
				return 1
			}
			mcGroupsMap, ok := mcGroups.(map[string]any)
			if !ok {
				fmt.Fprintf(stderr, "Error: managedClusterGroups is not a map\n")
				return 1
			}

			// First try direct key lookup, then search by name subfield
			if mcEntry, ok := mcGroupsMap[*withManagedCluster]; ok {
				spokeGroupKey = *withManagedCluster
				if mcEntryMap, ok := mcEntry.(map[string]any); ok {
					if name, ok := mcEntryMap["name"].(string); ok && name != "" {
						spokeGroupName = name
					}
				}
			}
			if spokeGroupName == "" {
				for key, entry := range mcGroupsMap {
					entryMap, ok := entry.(map[string]any)
					if !ok {
						continue
					}
					if name, ok := entryMap["name"].(string); ok && name == *withManagedCluster {
						spokeGroupKey = key
						spokeGroupName = name
						break
					}
				}
			}
			if spokeGroupName == "" {
				fmt.Fprintf(stderr, "Error: managed cluster group %q not found in managedClusterGroups (looked up by key and by name)\n", *withManagedCluster)
				return 1
			}
		}

		chartDirs, err := values.FindCharts(absPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error finding charts: %v\n", err)
			return 1
		}
		if len(chartDirs) == 0 && *withManagedCluster == "" {
			fmt.Fprintln(stdout, "\nNo charts found.")
			return 0
		}

		for _, chartDir := range chartDirs {
			rendered, err := values.RenderChart(chartDir, mergedValues)
			if err != nil {
				fmt.Fprintf(stderr, "Error rendering chart %s: %v\n", chartDir, err)
				return 1
			}
			names := make([]string, 0, len(rendered))
			for name := range rendered {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				content := rendered[name]
				if strings.TrimSpace(content) == "" {
					continue
				}
				fmt.Fprintf(stdout, "\n# %s\n", name)
				printStrippingComments(stdout, content)
			}
		}

		if *withManagedCluster != "" {
			fmt.Fprintf(stdout, "\n# Managed cluster: %s (%s), spoke: %s\n", spokeGroupKey, spokeGroupName, *spokeClusterName)

			spokeBaseFiles := values.BuildValueFiles(spokeGroupName, *clusterPlatform,
				*clusterVersion, *spokeClusterName, nil, absPath)

			fmt.Fprintln(stdout, "# Spoke base value files:")
			for _, f := range spokeBaseFiles {
				status := "exists "
				if _, err := os.Stat(f); os.IsNotExist(err) {
					status = "missing"
				}
				fmt.Fprintf(stdout, "#   [%s] %s\n", status, f)
			}

			spokeTemplateParams := make(map[string]string)
			if *clusterPlatform != "" {
				spokeTemplateParams["global.clusterPlatform"] = *clusterPlatform
			}
			if *clusterVersion != "" {
				spokeTemplateParams["global.clusterVersion"] = *clusterVersion
			}
			if *spokeClusterName != "" {
				spokeTemplateParams["global.clusterDomain"] = *spokeClusterName
			}

			spokeInput := values.ResolveInput{
				LocalCheckoutPath:  absPath,
				ClusterGroupName:   spokeGroupName,
				ClusterPlatform:    *clusterPlatform,
				ClusterVersion:     *clusterVersion,
				ClusterName:        *spokeClusterName,
				TemplateParameters: spokeTemplateParams,
			}

			spokeSharedFiles, err := values.ResolveSharedValueFiles(&spokeInput, absPath)
			if err != nil {
				fmt.Fprintf(stderr, "Error resolving spoke shared value files: %v\n", err)
				return 1
			}

			fmt.Fprintln(stdout)
			if len(spokeSharedFiles) == 0 {
				fmt.Fprintln(stdout, "# No spoke shared value files defined.")
			} else {
				fmt.Fprintln(stdout, "# Spoke shared value files:")
				for _, f := range spokeSharedFiles {
					fmt.Fprintf(stdout, "#   %s\n", f)
				}
			}

			var spokeExistingFiles []string
			for _, f := range append(spokeBaseFiles, spokeSharedFiles...) {
				if _, err := os.Stat(f); !os.IsNotExist(err) {
					spokeExistingFiles = append(spokeExistingFiles, f)
				}
			}

			spokeMergedValues, err := values.MergeHelmValues(spokeExistingFiles...)
			if err != nil {
				fmt.Fprintf(stderr, "Error merging spoke value files: %v\n", err)
				return 1
			}
			spokeCLIValues := buildRenderValues(spokeGroupName, *clusterPlatform, *clusterVersion,
				*spokeClusterName, *hubClusterDomain, *localClusterDomain, *clusterID)
			spokeMergedValues = chartutil.CoalesceTables(spokeCLIValues, spokeMergedValues)

			for _, chartDir := range chartDirs {
				rendered, err := values.RenderChart(chartDir, spokeMergedValues)
				if err != nil {
					fmt.Fprintf(stderr, "Error rendering spoke chart %s: %v\n", chartDir, err)
					return 1
				}
				names := make([]string, 0, len(rendered))
				for name := range rendered {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					content := rendered[name]
					if strings.TrimSpace(content) == "" {
						continue
					}
					fmt.Fprintf(stdout, "\n# spoke: %s\n", name)
					printStrippingComments(stdout, content)
				}
			}
		}
	}

	return 0
}

func printStrippingComments(w io.Writer, content string) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fmt.Fprintln(w, line)
	}
}

func buildRenderValues(clusterGroup, clusterPlatform, clusterVersion,
	clusterName, hubClusterDomain, localClusterDomain, clusterID string) map[string]any {
	global := make(map[string]any)
	if clusterPlatform != "" {
		global["clusterPlatform"] = clusterPlatform
	}
	if clusterVersion != "" {
		global["clusterVersion"] = clusterVersion
	}
	if clusterName != "" {
		global["clusterDomain"] = clusterName
		global["localClusterName"] = clusterName
	}
	if hubClusterDomain != "" {
		global["hubClusterDomain"] = hubClusterDomain
	}
	if localClusterDomain != "" {
		global["localClusterDomain"] = localClusterDomain
	}
	if clusterID != "" {
		global["clusterID"] = clusterID
	}

	result := map[string]any{
		"global": global,
	}
	if clusterGroup != "" {
		result["clusterGroup"] = map[string]any{
			"name": clusterGroup,
		}
	}
	return result
}
