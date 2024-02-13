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
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/strvals"
)

var settings *cli.EnvSettings

func Init() {
	settings = cli.New()
}

// RepoAdd adds repo with given name and url
func RepoAdd(name, url string) (bool, error) {
	repoFile := settings.RepositoryConfig

	// Ensure the file directory exists as it is required for file locking
	err := os.MkdirAll(filepath.Dir(repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return false, err
	}

	// Acquire a file lock for process synchronization
	fileLock := flock.New(strings.Replace(repoFile, filepath.Ext(repoFile), ".lock", 1))
	lockCtx, cancel := context.WithTimeout(context.Background(), TimeOut)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock() //nolint:errcheck
	}
	if err != nil {
		return false, err
	}

	b, err := os.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil { //nolint:govet
		return false, err
	}

	if f.Has(name) {
		fmt.Printf("repository name (%s) already exists\n", name)
		return false, nil
	}

	c := repo.Entry{
		Name: name,
		URL:  url,
	}

	_, err = repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return false, err
	}

	f.Update(&c)

	if err := f.WriteFile(repoFile, FilePerms); err != nil {
		return false, err
	}
	fmt.Printf("%q has been added to your repositories\n", name)
	return true, nil
}

// RepoUpdate updates charts for all helm repos
func RepoUpdate() (bool, error) {
	repoFile := settings.RepositoryConfig

	f, err := repo.LoadFile(repoFile)
	if os.IsNotExist(errors.Cause(err)) || len(f.Repositories) == 0 {
		return false, errors.New("no repositories found. You must add one before updating")
	}
	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return false, err
		}
		repos = append(repos, r)
	}

	fmt.Printf("Hang tight while we grab the latest from your chart repositories...\n")
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				fmt.Printf("...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
			} else {
				fmt.Printf("...Successfully got an update from the %q chart repository\n", re.Config.Name)
			}
		}(re)
	}
	wg.Wait()
	fmt.Printf("Update Complete. ⎈ Happy Helming!⎈\n")
	return true, nil
}

// InstallChart
func InstallChart(releaseName, repoName, chartName, version string, args map[string]string) (bool, error) {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debugf); err != nil {
		return false, err
	}
	client := action.NewInstall(actionConfig)

	if client.Version == "" && client.Devel {
		client.Version = ">0.0.0-0"
	}

	client.ReleaseName = releaseName
	client.Version = version
	cp, err := client.ChartPathOptions.LocateChart(fmt.Sprintf("%s/%s", repoName, chartName), settings)
	if err != nil {
		return false, err
	}

	p := getter.All(settings)
	valueOpts := &values.Options{}
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return false, err
	}

	// Add args
	if err = strvals.ParseInto(args["set"], vals); err != nil {
		return false, errors.Wrap(err, "failed parsing --set data")
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return false, err
	}

	validInstallableChart, err := isChartInstallable(chartRequested)
	if !validInstallableChart {
		return false, err
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err = action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              os.Stdout,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
				}
				if err := man.Update(); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
	}

	client.Namespace = settings.Namespace()
	releaseInfo, err := client.Run(chartRequested, vals)
	if err != nil {
		return false, err
	}

	fmt.Println("Installed Helm Chart [", releaseInfo.Name, "] in Namespace [", releaseInfo.Namespace, "]")

	return true, nil
}

// UnInstallChart
func UnInstallChart(name, namespace string) (bool, error) {
	if fDeployed, _ := isChartDeployed(name, namespace); fDeployed {
		actionConfig := new(action.Configuration)
		fmt.Println("Chart: ", name, " Namespace: ", namespace)
		if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debugf); err != nil {
			return false, err
		}

		client := action.NewUninstall(actionConfig)

		// Wait for chart to be uninstalled
		client.Wait = true

		// Uninstall the Helm chart
		releaseInfo, err := client.Run(name)
		if err != nil {
			return false, err
		}
		fmt.Println("Uninstalled Helm Chart [", releaseInfo.Release.Name, "] in Namespace [", releaseInfo.Release.Namespace, "]")
		return true, nil
	}
	return true, nil
}
func isChartDeployed(name, namespace string) (bool, error) {
	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Printf("%+v", err)
		return false, err
	}

	status := action.NewList(actionConfig)

	// We are only interested in deployed charts
	status.Deployed = true
	releases, err := status.Run()

	if err != nil {
		fmt.Println("Error from helm list:", err)
		return false, err
	}

	// Let's go through the deployed Helm Charts
	for _, rel := range releases {
		if rel.Name == name {
			log.Printf("Helm Chart %s found. Deployed in %s namespace\n", rel.Name, rel.Namespace)
			return true, nil
		}
	}

	log.Printf("Helm chart %s is not deployed", name)
	return false, nil
}

func getChartRelease(name, namespace string) (*release.Release, error) {
	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Printf("%+v", err)
		return nil, err
	}

	status := action.NewList(actionConfig)

	// We are only interested in deployed charts
	status.Deployed = true
	releases, err := status.Run()

	if err == nil {
		// Let's go through the deployed Helm Charts
		for _, rel := range releases {
			if rel.Name == name {
				log.Printf("Helm Chart %s found. Deployed in %s namespace\n", rel.Name, rel.Namespace)
				return rel, nil
			}
		}
	}
	return nil, err
}
func isChartInstallable(ch *chart.Chart) (bool, error) {
	switch ch.Metadata.Type {
	case "", "application":
		return true, nil
	}
	return false, errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

//nolint:gofmt
func debugf(format string, v ...interface{}) {
	format = fmt.Sprintf("[debug] %s\n", format)
	log.Output(2, fmt.Sprintf(format, v...)) //nolint:errcheck
}
