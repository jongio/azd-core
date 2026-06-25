// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package keyvault

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

func TestResolveBySecretURI_InvalidFormats(t *testing.T) {
	cases := []struct {
		name string
		uri  string
	}{
		{"empty string", ""},
		{"no scheme", "vault.azure.net/secrets/mysecret"},
		{"wrong host", "https://example.com/secrets/mysecret"},
		{"missing secret name", "https://myvault.vault.azure.net/secrets/"},
		{"no path segments", "https://myvault.vault.azure.net"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &KeyVaultResolver{
				credential: nil,
				clients:    make(map[string]*azsecrets.Client),
			}
			_, err := r.resolveBySecretURI(context.Background(), tc.uri)
			if err == nil {
				t.Errorf("resolveBySecretURI(%q) expected error, got nil", tc.uri)
			}
		})
	}
}

func TestValidateVaultName_Coverage(t *testing.T) {
	cases := []struct {
		name      string
		vaultName string
		wantErr   bool
	}{
		{"valid name", "my-vault-01", false},
		{"too short", "ab", true},
		{"too long", strings.Repeat("a", 25), true},
		{"invalid chars", "my_vault!", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateVaultName(tc.vaultName)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateVaultName(%q) error = %v, wantErr %v", tc.vaultName, err, tc.wantErr)
			}
		})
	}
}

func TestResolveReference_InvalidFormats(t *testing.T) {
	cases := []struct {
		name string
		ref  string
	}{
		{"empty ref", ""},
		{"no vault prefix", "secret/mysecret"},
		{"missing secret after vault", "vault/myvault"},
		{"triple slash", "vault///secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &KeyVaultResolver{
				credential: nil,
				clients:    make(map[string]*azsecrets.Client),
			}
			_, err := r.ResolveReference(context.Background(), tc.ref)
			if err == nil {
				t.Errorf("ResolveReference(%q) expected error, got nil", tc.ref)
			}
		})
	}
}

func TestResolveBySecretURI_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &KeyVaultResolver{
		credential: nil,
		clients:    make(map[string]*azsecrets.Client),
	}
	_, err := r.resolveBySecretURI(ctx, "https://myvault.vault.azure.net/secrets/mysecret")
	if err == nil {
		t.Error("resolveBySecretURI with canceled context expected error, got nil")
	}
}

func TestResolveReference_NonKVPassthrough(t *testing.T) {
	r := &KeyVaultResolver{
		credential: nil,
		clients:    make(map[string]*azsecrets.Client),
	}
	_, err := r.ResolveReference(context.Background(), "not-a-kv-reference")
	if err == nil {
		t.Error("ResolveReference with non-KV reference expected error")
	}
}

func TestResolveByVaultNameAndSecret_InvalidNames(t *testing.T) {
	cases := []struct {
		name       string
		vaultName  string
		secretName string
	}{
		{"empty vault", "", "mysecret"},
		{"empty secret", "myvault", ""},
		{"invalid vault name", "x", "mysecret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &KeyVaultResolver{
				credential: nil,
				clients:    make(map[string]*azsecrets.Client),
			}
			_, err := r.resolveByVaultNameAndSecret(context.Background(), tc.vaultName, tc.secretName, "")
			if err == nil {
				t.Errorf("resolveByVaultNameAndSecret(%q, %q) expected error", tc.vaultName, tc.secretName)
			}
		})
	}
}
