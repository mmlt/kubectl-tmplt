package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/mmlt/kubectl-tmplt/pkg/util/backoff"
	"time"

	//"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest/azure"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

// https://docs.microsoft.com/en-us/azure/key-vault/about-keys-secrets-and-certificates

// ResourceKeyVault is the resource name for KeyVault.
// (also referred to as 'audience')
const resourceKeyVault = "https://vault.azure.net"

// NewKeyVault returns an Azure Key Vault client.
// Values contains "URL" and client credentials (or certificates or username/password)
// as documented in https://docs.microsoft.com/en-us/azure/go/azure-sdk-go-authorization#use-environment-based-authentication
// During development cli=true can be specified to use `az login` bearer token instead.
func NewKeyVault(values map[string]string) (*KV, error) {
	k := &KV{
		client: keyvault.New(),
		url:    strings.TrimSuffix(values["URL"], "/"),
	}

	if k.url == "" {
		return nil, fmt.Errorf("no URL")
	}

	var a autorest.Authorizer
	var err error
	switch values["cli"] {
	case "true":
		a, err = auth.NewAuthorizerFromCLIWithResource(resourceKeyVault)
	default:
		a, err = getAuthorizerFrom(values)
	}
	if err != nil {
		return nil, err
	}

	k.client.Authorizer = a

	return k, nil
}

// KV provides reading of secrets from Azure Key Vaults.
type KV struct {
	client keyvault.BaseClient
	url    string
}

// GetAuthorizerFrom returns an authorizer from VAULT_* key-value pairs.
func getAuthorizerFrom(values map[string]string) (autorest.Authorizer, error) {
	s, err := getSettingsFrom(values)
	if err != nil {
		return nil, err
	}

	a, err := s.GetAuthorizer()
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Get value addressed by key from vault.
// If field is empty return the value as-is.
// Otherwise expect the value to be a JSON object and field a field of the object.
func (k KV) Get(key, field string) string {
	s, err := k.get(key)
	if err != nil {
		return err.Error()
	}

	if field == "" || field == "." {
		return s
	}

	m := map[string]string{}
	err = json.Unmarshal([]byte(s), &m)
	if err != nil {
		return err.Error()
	}

	if v := m[field]; v != "" {
		return v
	}

	return fmt.Sprintf("no field %s in secret %s", field, key)
}

// Get gets a KeyVault secret by name.
func (k KV) get(name string) (string, error) {
	var err error
	for exp := backoff.NewExponential(10 * time.Second); exp.Retries() < 10; exp.Sleep() {
		r, e := k.client.GetSecret(context.Background(), k.url, name, "")
		if e == nil {
			return *r.Value, nil
		}
		err = e
	}
	return "", fmt.Errorf("no secret %s: %w", name, err)
}

func getSettingsFrom(values map[string]string) (*auth.EnvironmentSettings, error) {
	clientCredentials := []string{auth.TenantID, auth.ClientID, auth.ClientSecret}
	certificate := []string{auth.TenantID, auth.ClientID, auth.CertificatePath, auth.CertificatePassword}
	usernamePassword := []string{auth.TenantID, auth.ClientID, auth.Username, auth.Password}
	if !(has(values, clientCredentials) ||
		has(values, certificate) ||
		has(values, usernamePassword)) {
		return nil, fmt.Errorf("expected client, certificate or username/password credential values")
	}

	s := &auth.EnvironmentSettings{
		Values:      values,
		Environment: azure.PublicCloud,
	}

	var err error
	if v := values[auth.EnvironmentName]; v != "" {
		s.Environment, err = azure.EnvironmentFromName(v)
	}
	if values[auth.Resource] == "" {
		s.Values[auth.Resource] = resourceKeyVault
	}

	return s, err
}

// Has returns true if m has required values.
func has(m map[string]string, required []string) bool {
	for _, r := range required {
		if m[r] == "" {
			return false
		}
	}
	return true
}
