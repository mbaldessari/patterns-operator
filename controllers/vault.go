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
// Note: We only keep the minimum needed fields around
type VaultInitStruct struct {
	UnsealKeysHex   []string `json:"unseal_keys_hex"`
	UnsealShares    int      `json:"unseal_shares"`
	UnsealThreshold int      `json:"unseal_threshold"`
	RootToken       string   `json:"root_token"`
}

// {
// 	"type": "shamir",
// 	"initialized": false,
// 	"sealed": true,
// 	"t": 0,
// 	"n": 0,
// 	"progress": 0,
// 	"nonce": "",
// 	"version": "1.9.2",
// 	"migration": false,
// 	"recovery_seal": false,
// 	"storage_type": "file",
// 	"ha_enabled": false,
// 	"active_time": "0001-01-01T00:00:00Z"
//   }
type VaultStatus struct {
	Type        string `json:"type"`
	Initialized bool   `json:"initialized"`
	Sealed      bool   `json:"sealed"`
	T           int    `json:"t"`
	N           int    `json:"n"`
	Progress    int    `json:"progress"`
	Version     string `json:"version"`
	StorageType string `json:"storage_type"`
	HaEnabled   bool   `json:"ha_enabled"`
}

const vaultSecretName string = "vaultkeys"

func vaultOperatorInit(config *rest.Config, client kubernetes.Interface) (*VaultInitStruct, error) {
	stdout, _, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "operator", "init", "-format=json"})
	if err != nil {
		return nil, err
	}
	var unmarshalled VaultInitStruct
	err = json.Unmarshal(stdout.Bytes(), &unmarshalled)
	if err != nil {
		return nil, err
	}
	return &unmarshalled, nil
}

func addVaultSecrets(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultInitStruct) error {
	log.Printf("Adding vault keys to secrets")
	data := map[string][]byte{
		"roottoken": []byte(vaultInitOutput.RootToken),
	}
	for index, key := range vaultInitOutput.UnsealKeysHex {
		s := "unsealhexkey_" + strconv.Itoa(index)
		data[s] = []byte(key)
	}
	data["keyscount"] = []byte(strconv.Itoa(len(vaultInitOutput.UnsealKeysHex)))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: vaultSecretName,
		},
		Data: data,
	}
	// FIXME: this is not always 'patterns-operator-system' it is 'openshift-operators' when installing via UI
	secretClient := client.CoreV1().Secrets("patterns-operator-system")
	secret, err := secretClient.Get(context.Background(), vaultSecretName, metav1.GetOptions{})
	if err != nil || secret == nil {
		_, err = secretClient.Create(context.Background(), secret, metav1.CreateOptions{})
	} else {
		_, err = secretClient.Update(context.Background(), secret, metav1.UpdateOptions{})
	}
	if err != nil {
		log.Printf("Error creating secret: %s\n", err)
		return err
	}
	log.Printf("Created secret")
	return nil
}

func getVaultStructFromSecrets(config *rest.Config, client kubernetes.Interface) (*VaultInitStruct, error) {
	// If the vault is sealed we take the unseal keys in the k8s secret and use them to unseal the vault
	// FIXME: this is not always 'patterns-operator-system' it is 'openshift-operators' when installing via UI
	secretClient := client.CoreV1().Secrets("patterns-operator-system")
	secret, err := secretClient.Get(context.Background(), "vaultkeys", metav1.GetOptions{})
	if err != nil || secret == nil {
		return nil, errors.New(fmt.Errorf("We called vaultUnseal but there were no secrets present: %s", err))
	}
	count, err := strconv.Atoi(string(secret.Data["keyscount"]))
	if err != nil {
		return nil, errors.New(fmt.Errorf("Converting keys count failed: %s", err))
	}
	var v VaultInitStruct
	v.RootToken = string(secret.Data["roottoken"])
	v.UnsealKeysHex = []string{}
	for i := 0; i < count; i++ {
		v.UnsealKeysHex = append(v.UnsealKeysHex, string(secret.Data["unsealhexkey_"+strconv.Itoa(i)]))
	}
	err = unsealVaultOperator(config, client, &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func unsealVaultOperator(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultInitStruct) error {
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

func vaultStatus(config *rest.Config, client kubernetes.Interface) (*VaultStatus, error) {
	if haveNamespace(client, "vault") == false {
		return nil, errors.New(fmt.Errorf("'vault' namespace not found yet"))
	}
	if havePod(client, "vault", "vault-0") == false {
		return nil, errors.New(fmt.Errorf("'vault/vault-0' pod not found yet"))
	}
	log.Printf("vault/vault-0 exists. Getting vault status:")
	stdout, _, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "status", "-format=json"})
	if err != nil {
		return nil, err
	}
	var unmarshalled VaultStatus
	err = json.Unmarshal(stdout.Bytes(), &unmarshalled)
	if err != nil {
		return nil, err
	}
	return &unmarshalled, nil
}

func vaultInitialize(config *rest.Config, client kubernetes.Interface) error {
	status, err := vaultStatus(config, client)
	if err != nil {
		return err
	}
	if status.Initialized == true {
		return nil
	}
	// If the vault is not initialized we call 'vault operator init -format=json' and store the unseal keys in k8s
	vaultKeys, err := vaultOperatorInit(config, client)
	if err != nil {
		return err
	}

	// Let's store the keys into a secret
	if err = addVaultSecrets(config, client, vaultKeys); err != nil {
		return err
	}
	return nil
}

func vaultUnseal(config *rest.Config, client kubernetes.Interface) error {
	status, err := vaultStatus(config, client)
	if err != nil {
		return err
	}
	if status.Sealed == false {
		return nil
	}
	if status.Initialized == false {
		return errors.New("Vault is sealed but not initialized. This is a non-expected state!")
	}
	v, err := getVaultStructFromSecrets(config, client)
	if err != nil {
		return err
	}
	err = unsealVaultOperator(config, client, v)
	return err
}

func vaultLogin(config *rest.Config, client kubernetes.Interface) error {
	stdout, stderr, err := execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "token", "lookup"})
	// we are already logged in. Nothing else to do here
	if err == nil {
		return nil
	}
	var ret int = 0
	if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
		ret = exitErr.ExitStatus()
	}
	// There has been a generic error while looking up the token
	if ret == 1 || ret > 2 {
		return errors.New(fmt.Errorf("Generic error while looking up vault token: %s,%s", stdout, stderr))
	}
	v, err := getVaultStructFromSecrets(config, client)
	if err != nil {
		return err
	}
	// The user does not have a token so we must login using the root token
	stdout, stderr, err = execInPod(config, client, "vault", "vault-0", "vault", []string{"vault", "login", v.RootToken})
	if err != nil {
		log.Printf("Error while logging in to the vault %s %s %s\n", stdout.String(), stderr.String(), err)
		return err
	}
	log.Printf("Logged into the vault successfully")
	return nil
}
