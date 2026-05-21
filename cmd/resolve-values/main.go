package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hybrid-cloud-patterns/patterns-operator/pkg/values"
)

func main() {
	repoPath := flag.String("path", ".", "Path to the patterns git repo checkout")
	clusterGroup := flag.String("cluster-group", "hub", "Cluster group name")
	clusterPlatform := flag.String("cluster-platform", "", "Cluster platform (e.g. AWS, Azure)")
	clusterVersion := flag.String("cluster-version", "", "Cluster version (e.g. 4.12)")
	clusterName := flag.String("cluster-name", "", "Cluster name")
	extraValueFiles := flag.String("extra-value-files", "", "Comma-separated extra value file paths")
	flag.Parse()

	absPath, err := filepath.Abs(*repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absPath); err != nil {
		fmt.Fprintf(os.Stderr, "Path does not exist: %s\n", absPath)
		os.Exit(1)
	}

	var extras []string
	if *extraValueFiles != "" {
		extras = strings.Split(*extraValueFiles, ",")
	}

	baseFiles := values.BuildValueFiles(*clusterGroup, *clusterPlatform,
		*clusterVersion, *clusterName, extras, absPath)

	fmt.Println("Base value files:")
	for _, f := range baseFiles {
		status := "exists "
		if _, err := os.Stat(f); os.IsNotExist(err) {
			status = "missing"
		}
		fmt.Printf("  [%s] %s\n", status, f)
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
		fmt.Fprintf(os.Stderr, "Error resolving shared value files: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	if len(sharedFiles) == 0 {
		fmt.Println("No shared value files defined.")
	} else {
		fmt.Println("Shared value files (from clusterGroup.sharedValueFiles):")
		for _, f := range sharedFiles {
			fmt.Printf("  %s\n", f)
		}
	}
}
