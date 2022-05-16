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
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/go-errors/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	utilexec "k8s.io/client-go/util/exec"
)

//
// {
// 	"unseal_keys_b64": [
// 	  "R/BL306DUjRQIHdkYYxheqFxr6PtZVEKtHaYNjFqBGq7",
// 	  "+4CYavmqRWq165WJM4DqpnEqlDnECt6q+6jSmEaJBsBA",
// 	  "tlsQ833l5k52ESK28jlZlWbegBRY+HNIJD9Yqp3cEdF6",
// 	  "ON0wQUleo+iW4r6U0EwmoOkRhezzTke09h+rxgRDPkdo",
// 	  "i1hmENQhAcq5t6WWxTR35YDAUjY1w8ry751CggPsB0Jk"
// 	],
// 	"unseal_keys_hex": [
// 	  "47f04bdf4e83523450207764618c617aa171afa3ed65510ab4769836316a046abb",
// 	  "fb80986af9aa456ab5eb95893380eaa6712a9439c40adeaafba8d298468906c040",
// 	  "b65b10f37de5e64e761122b6f239599566de801458f87348243f58aa9ddc11d17a",
// 	  "38dd3041495ea3e896e2be94d04c26a0e91185ecf34e47b4f61fabc604433e4768",
// 	  "8b586610d42101cab9b7a596c53477e580c0523635c3caf2ef9d428203ec074264"
// 	],
// 	"unseal_shares": 5,
// 	"unseal_threshold": 3,
// 	"recovery_keys_b64": [],
// 	"recovery_keys_hex": [],
// 	"recovery_keys_shares": 5,
// 	"recovery_keys_threshold": 3,
// 	"root_token": "s.lAR1G890NPBEkzRt8Ic5kBVz"
//   }
type VaultOperatorInit struct {
	UnsealKeysB64   []string `json:"unseal_keys_b64"`
	UnsealKeysHex   []string `json:"unseal_keys_hex"`
	UnsealShares    int      `json:"unseal_shares"`
	UnsealThreshold int      `json:"unseal_threshold"`
	RootToken       string   `json:"root_token"`
}

func initVaultOperator(config *rest.Config, client kubernetes.Interface) (*VaultOperatorInit, error) {
	stdout, _, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "operator", "init", "-format=json"})
	if err != nil {
		return nil, err
	}
	var unmarshalled VaultOperatorInit
	err = json.Unmarshal(stdout.Bytes(), &unmarshalled)
	if err != nil {
		return nil, err
	}
	return &unmarshalled, nil
}

func addVaultSecrets(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultOperatorInit) error {
	log.Printf("Adding vault keys to secrets")
	data := map[string][]byte{
		"roottoken": []byte(vaultInitOutput.RootToken),
	}
	for index, key := range vaultInitOutput.UnsealKeysHex {
		s := key + "_" + strconv.Itoa(index)
		data[s] = []byte(key)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vault_keys",
			//Labels: rc.ObjectMeta.Labels,
		},
	}
	secretClient := client.CoreV1().Secrets("pattern-operator-system")
	current, err := secretClient.Get(context.Background(), "vault_keys", metav1.GetOptions{})
	if err != nil || current == nil {
		_, err = secretClient.Create(context.Background(), secret, metav1.CreateOptions{})
	} else {
		_, err = secretClient.Update(context.Background(), secret, metav1.UpdateOptions{})
	}
	if err != nil {
		return err
	}
	log.Printf("Created secret")
	return nil
}

func unsealVaultOperator(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultOperatorInit) error {
	var errCount int = 0
	if len(vaultInitOutput.UnsealKeysHex) == 0 || len(vaultInitOutput.UnsealKeysHex) < vaultInitOutput.UnsealThreshold {
		return errors.New("We do not have sufficient keys to unseal the vault")
	}

	for _, key := range vaultInitOutput.UnsealKeysHex {
		_, _, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "operator", "unseal", key})
		if err != nil {
			errCount += 1
			log.Printf("Error while processing %s: -> %s", key, err)
		}
	}
	if errCount > 0 {
		return errors.New("Errored while calling vault operator unseal")
	}

	log.Printf("Vault successfully unsealed")
	return nil
}

func loginVault(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultOperatorInit) error {
	stdout, stderr, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "login", vaultInitOutput.RootToken})
	if err != nil {
		log.Printf("Error while logging in to the vault %s %s %s\n", stdout.String(), stderr.String(), err)
		return err
	}
	log.Printf("Logged into the vault successfully")
	return nil
}

func unsealVault(config *rest.Config, client kubernetes.Interface) error {
	if haveNamespace(client, "vault") == false {
		return errors.New(fmt.Errorf("'vault' namespace not found yet"))
	}
	if havePod(client, "vault", "vault-0") == false {
		return errors.New(fmt.Errorf("'vault/vault-0' pod not found yet"))
	}
	log.Printf("vault/vault-0 exists. Getting vault status:")

	stdout, stderr, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "status"})
	var ret int = 0
	if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
		ret = exitErr.ExitStatus()
	}
	switch ret {
	case 2: // vault is sealed
		vaultInitOutput, err := initVaultOperator(config, client)
		if err != nil {
			return err
		}
		// store unseal keys + root token in a secret
		addVaultSecrets(config, client, vaultInitOutput)
		if err := unsealVaultOperator(config, client, vaultInitOutput); err != nil {
			return err
		}
		// Now the vault is unsealed we only need to log into it
		if err := loginVault(config, client, vaultInitOutput); err != nil {
			return err
		}
		log.Printf("Vault is all unsealed and logged in")
	case 1: // vault status returned error
		log.Printf("Vault status returned error 1. %s, %s", stdout.String(), stderr.String())
		return err
	case 0: // vault is unsealed and ok
		log.Printf("Vault status returned ok. %s, %s", stdout.String(), stderr.String())
		return nil
	}

	log.Printf("Vault status returned an unexpected state: %s, %s", stdout.String(), stderr.String())
	return err
}
