/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"net/http"

	"code.gitea.io/sdk/gitea"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/hybrid-cloud-patterns/patterns-operator/api/v1alpha1"
)

// Function that creates a mirror repo in Gitea
func migrateGiteaRepo(fullClient kubernetes.Interface, username, password, upstreamURL, giteaServerRoute string) (success bool, repositoryURL string, err error) {
	option := gitea.SetBasicAuth(username, password)
	httpClient := &http.Client{
		Transport: getHTTPSTransport(fullClient),
	}

	giteaClient, err := gitea.NewClient(giteaServerRoute, option, gitea.SetHTTPClient(httpClient))
	if err != nil {
		return false, "", err
	}

	// Let's extract the repo name
	repoName, _ := extractRepositoryName(upstreamURL)

	// Check to see if the repo already exists
	repository, response, _ := giteaClient.GetRepo(GiteaAdminUser, repoName)

	// Repo has been already migrated
	if response.Response.StatusCode == http.StatusOK {
		return true, repository.HTMLURL, nil
	}

	// Default description will include repo name and that it was created by
	// the Validated Patterns operator.
	descriptionFormat := "The [%s] repository was migrated by the Validated Patterns Operator."

	description := fmt.Sprintf(descriptionFormat, repoName)

	repository, _, err = giteaClient.MigrateRepo(gitea.MigrateRepoOption{
		CloneAddr: upstreamURL,
		RepoOwner: username,
		RepoName:  repoName,
		Mirror:    true, // We want gitea to periodically check for changes so create a mirror
		// Private:     true, // TODO: uncomment this if you want the repo to be private on gitea
		Description: description,
	})

	if err != nil {
		return false, "", err
	}

	return true, repository.HTMLURL, nil
}

func hasGiteaInstance(c client.Client) (exists bool, err error) {
	// First let's see if there's a GiteaServer instance
	// We list all the instances of a GiteaServer
	// We don't care what the name of the instance is. If there's one present
	// we assume that the Gitea Server instance is operational.
	listOpts := client.ListOptions{
		Namespace: GiteaNamespace,
	}
	giteaServerInstanceList := &api.GiteaServerList{}
	err = c.List(context.Background(), giteaServerInstanceList, &listOpts)
	if err != nil && kerrors.IsNotFound(err) {
		return false, nil
	} else if err != nil && !kerrors.IsNotFound(err) {
		return false, err
	}
	return true, nil
}

func createGiteaInstance(c client.Client) error {
	giteaServerInstance := &api.GiteaServer{
		ObjectMeta: metav1.ObjectMeta{Name: GiteaServerDefaultName, Namespace: GiteaNamespace},
		Spec: api.GiteaServerSpec{
			HelmChartUrl:     GiteaHelmRepoUrl,
			HelmRepoName:     GiteaRepoName,
			HelmChartName:    GiteaChartName,
			HelmChartVersion: GiteaDefaultChartVersion,
			HelmReleaseName:  GiteaReleaseName,
		},
	}
	return c.Create(context.Background(), giteaServerInstance)
}
