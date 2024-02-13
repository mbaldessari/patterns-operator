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
)

// GitOps Subscription
const (
	GitOpsDefaultChannel                = "gitops-1.11"
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
	GiteaHelmRepoUrl = "https://charts.validatedpatterns.io/"
	// Repo name for the Validated Patterns Helm repo
	GiteaRepoName = "helm-charts"
	// Gitea chart name in the Validated Patterns repo
	GiteaChartName = "gitea-chart"
	// Release name used by the Helm SDK
	GiteaReleaseName = "gitea"
	// Namespace for the Gitea resources
	GiteaNamespace = "vp-gitea"
	// Our gitea-chart default version
	GiteaDefaultChartVersion = "0.0.3"
	// Default Gitea Admin user
	GiteaAdminUser = "gitea_admin"
	// Gitea Admin Secrets name
	GiteaAdminSecretName = "gitea-admin-secret"
	// GiteaServer default name
	GiteaServerDefaultName = "vp-gitea-instance"
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
