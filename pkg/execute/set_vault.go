package execute

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hashicorp/vault/api"
	"github.com/mmlt/kubectl-tmplt/pkg/util/backoff"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"gopkg.in/yaml.v3"
	"strings"
	"time"
)

// SetVault is an Action to set configuration in a Vault in the target cluster.
func (x *Execute) setVault(id int, name string, doc []byte, portForward string, passedValues *yamlx.Values) error {
	// get action arguments
	av := &actionVault{}
	err := yaml.Unmarshal(doc, av)
	if err != nil {
		return fmt.Errorf("setVault: %w", err)
	}

	//TODO consider validation on URL Token containing "" and (CA=="" ICW tlsSkipVerify=="true")

	vault, err := newVaultClient(av.URL, av.CA, av.TLSSkipVerify == "true", av.Token)
	if err != nil {
		return err
	}

	if portForward != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		x.kubectlPortForward(ctx, portForward)

		x.waitForVaultUp(ctx, vault)
		if ctx.Err() != nil {
			return fmt.Errorf("waiting for Vault: %w", ctx.Err())
		}
	}

	err = x.vaultLogicalConfig(vault, av.Config.Logicals)
	if err != nil {
		return err
	}
	err = x.vaultPolicyConfig(vault, av.Config.Policies)
	if err != nil {
		return err
	}
	// for backwards compatibility, will be removed
	if av.Config.KV != nil {
		err = x.vaultConfigKV(vault, av.Config.KV)
		if err != nil {
			return err
		}
	}

	return nil
}

type actionVault struct {
	Type          string      `yaml:"type"`
	URL           string      `yaml:"url"`
	Token         string      `yaml:"token"`
	TLSSkipVerify string      `yaml:"tlsSkipVerify"`
	CA            string      `yaml:"ca"`
	Config        vaultConfig `yaml:"config"`
}

// VaultConfig contains the data to write to Vault.
type vaultConfig struct {
	// Logical configuration
	// logical:
	// - path: secret/data/infra/iitech/exdns
	//   data:
	//     data:
	Logicals []logicalItem `yaml:"logicals"`
	// Policy configuration
	// policy:
	// - name: policy-name
	//   rule: policy
	Policies []policyItem `yaml:"policies"`

	// KV is for backwards compatibility, will be removed, use logical instead
	KV []logicalItem `yaml:"kv"`
}

type logicalItem struct {
	Path string                 `yaml:"path"`
	Data map[string]interface{} `yaml:"data"`
}

type policyItem struct {
	Name string `yaml:"name"`
	Rule string `yaml:"rule"`
}

// KubectlPortForward starts a port-forward until context cancel or timeout.
// On error the kubectl port-forward is retried.
func (x *Execute) kubectlPortForward(ctx context.Context, flags string) {
	args := strings.Split(flags, " ")
	args = append([]string{"port-forward"}, args...)
	go func(c context.Context, l logr.Logger, k Kubectler, a []string) {
		exp := backoff.NewExponential(10 * time.Second)
		for {
			_, _, _ = k.Run(c, "", a...) // error is logged
			if c.Err() != nil {
				// context cancelled or timeout
				return
			}
			exp.Sleep()
			l.V(4).Info("retry port-forward")
		}
	}(ctx, x.Log, x.Kubectl, args)
}

// NewVaultClient returns a client to access Hashi Corp Vault.
func newVaultClient(url, ca string, insecure bool, token string) (*api.Client, error) {
	c := api.DefaultConfig()
	c.Address = url
	err := c.ConfigureTLS(&api.TLSConfig{
		CACert:   ca,
		Insecure: insecure,
	})

	clnt, err := api.NewClient(c)
	if err != nil {
		return nil, err
	}

	clnt.SetToken(token)

	//TODO configure client to retry
	//clnt.SetBackoff()
	//clnt.SetMaxRetries()

	return clnt, nil
}

func (x *Execute) waitForVaultUp(ctx context.Context, vault *api.Client) {
	exp := backoff.NewExponential(10 * time.Second)
	for {
		h, err := vault.Sys().Health()
		if err == nil && h.Initialized {
			// success
			return
		}
		exp.Sleep()
		x.Log.V(4).Info("retry wait for Vault up")
	}
}

// VaultConfigKV writes KV items to vault.
// Deprecated, use VaultLogicalConfig
func (x *Execute) vaultConfigKV(vault *api.Client, kv []logicalItem) error {
	for _, item := range kv {
		var err error
		x.Log.V(4).Info("Vault write", "path", item.Path)
		exp := backoff.NewExponential(10 * time.Second)
		for exp.Retries() < 10 {
			_, err = vault.Logical().Write(item.Path, item.Data)
			if err == nil {
				break
			}
			exp.Sleep()
			x.Log.V(4).Info("Vault write error, retrying", "error", err)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// VaultLogicalConfig writes logical items to vault.
func (x *Execute) vaultLogicalConfig(vault *api.Client, items []logicalItem) error {
	for _, item := range items {
		var err error
		x.Log.V(4).Info("Vault write logical", "path", item.Path)
		exp := backoff.NewExponential(10 * time.Second)
		for exp.Retries() < 10 {
			_, err = vault.Logical().Write(item.Path, item.Data)
			if err == nil {
				break
			}
			exp.Sleep()
			x.Log.V(4).Info("Vault write error, retrying", "error", err)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// VaultPolicyConfig writes policies to vault.
func (x *Execute) vaultPolicyConfig(vault *api.Client, items []policyItem) error {
	for _, item := range items {
		var err error
		x.Log.V(4).Info("Vault write policy", "name", item.Name)
		exp := backoff.NewExponential(10 * time.Second)
		for exp.Retries() < 10 {
			err = vault.Sys().PutPolicy(item.Name, item.Rule)
			if err == nil {
				break
			}
			exp.Sleep()
			x.Log.V(4).Info("Vault write policy error, retrying", "error", err)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

//TODO create set_vault_test.go
