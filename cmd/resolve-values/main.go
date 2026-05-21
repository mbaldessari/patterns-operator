package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hybrid-cloud-patterns/patterns-operator/pkg/values"
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
	extraValueFiles := fs.String("extra-value-files", "", "Comma-separated extra value file paths")

	if err := fs.Parse(args); err != nil {
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

	sharedFiles, err := values.ResolveSharedValueFiles(input, absPath)
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
	return 0
}
