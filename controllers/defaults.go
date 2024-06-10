package controllers

import "time"

// Reconcile look requeue time
const ReconcileLoopRequeueTime = 180 * time.Second

// Below are the default constants that we will
// use throughout the patterns operator code
const (
	// Default Operator Namespace
	OperatorNamespace = "openshift-operators"
	// Default Operator Config Map Name
	OperatorConfigMap = "patterns-operator-config"
	// Default Subscription Namespace
	SubscriptionNamespace = "openshift-operators"
	// Default Application Namespace
	ApplicationNamespace = "openshift-gitops"
	// ClusterWide Argo Name
	ClusterWideArgoName = "openshift-gitops"
)

// GitOps Subscription
const (
	GitOpsDefaultChannel                = "gitops-1.12"
	GitOpsDefaultPackageName            = "openshift-gitops-operator"
	GitOpsDefaultCatalogSource          = "redhat-operators"
	GitOpsDefaultCatalogSourceNamespace = "openshift-marketplace"
	GitOpsDefaultApprovalPlan           = "Automatic"
)

// GitOps Configuration
const (
	// Require manual intervention before Argo will sync new content. Default: False
	GitOpsDefaultManualSync = "false"
	// Require manual confirmation before installing and upgrading operators. Default: False
	GitOpsDefaultManualApproval = "false"
	// Dangerous. Force a specific version to be installed. Default: False
	GitOpsDefaultUseCSV = "false"
)

// Experimental Capabilities that can be enabled
// Currently none

var DefaultPatternOperatorConfig = map[string]string{
	"gitops.catalogSource":       GitOpsDefaultCatalogSource,
	"gitops.name":                GitOpsDefaultPackageName,
	"gitops.channel":             GitOpsDefaultChannel,
	"gitops.sourceNamespace":     GitOpsDefaultCatalogSourceNamespace,
	"gitops.installApprovalPlan": GitOpsDefaultApprovalPlan,
	"gitops.ManualSync":          GitOpsDefaultManualSync,
}

type GitOpsConfig map[string]string

var PatternsOperatorConfig GitOpsConfig

// Gitea chart defaults
const (
	// URL to the Validated Patterns Helm chart repo
	Helm_Chart_Repo_URL = "https://charts.validatedpatterns.io/"
	// Repo name for the Validated Patterns Helm repo
	RepoName = "helm-charts"
	// Gitea chart name in the Validated Patterns repo
	ChartName = "gitea-chart"
	// Release name used by the Helm SDK
	ReleaseName = "gitea"
	// Namespace for the Gitea resources
	Gitea_Namespace = "vp-gitea"
	// Our gitea-chart default version
	Gitea_Default_Version = "0.0.3"
	// Default Gitea Admin user
	Gitea_Admin_User = "gitea_admin"
	// Gitea Admin Secrets name
	Gitea_Admin_Secret_Name = "gitea-admin-secret"
	// GiteaServer default name
	GiteaServer_Default_Name = "vp-gitea-instance"
)

// Default constants
const (
	FilePerms          = 0644
	TimeOut            = 30 * time.Second
	DefaultPasswordLen = 15
)

func (g GitOpsConfig) getValueWithDefault(k string) string {
	if v, present := g[k]; present {
		return v
	}
	if defaultValue, present := DefaultPatternOperatorConfig[k]; present {
		return defaultValue
	}
	return ""
}
