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
	"fmt"
	"log"

	"github.com/go-errors/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func unsealVault(config *rest.Config, client kubernetes.Interface) error {

	if haveNamespace(client, "vault") == false {
		return errors.New(fmt.Errorf("'vault' namespace not found yet"))
	}
	if havePod(client, "vault", "vault-0") == false {
		return errors.New(fmt.Errorf("'vault/vault-0' pod not found yet"))
	}
	log.Printf("vault/vault-0 exists. Getting vault status:")
	cmd := []string{
		"vault",
		"status",
	}
	execInPod(config, client, "vault", "vault-0", "vault", cmd)
	return nil
}
