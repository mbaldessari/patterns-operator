package controllers

import (
	"github.com/hybrid-cloud-patterns/patterns-operator/pkg/values"
)

func mergeHelmValues(files ...string) (map[string]any, error) {
	return values.MergeHelmValues(files...)
}

func getClusterGroupValue(key string, v map[string]any) any {
	return values.GetClusterGroupValue(key, v)
}

func helmTpl(templateString string, valueFiles []string, v map[string]any) (string, error) {
	return values.HelmTpl(templateString, valueFiles, v)
}

func countApplicationsAndSets(a any) (appCount, appSetsCount int) {
	return values.CountApplicationsAndSets(a)
}
