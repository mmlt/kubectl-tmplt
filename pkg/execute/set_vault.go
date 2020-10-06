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

	//err = vaultConfigPolicies(vault, av.Config.Policies)
	//if err != nil {
	//	return err
	//}

	err = x.vaultConfigKV(vault, av.Config.KV)
	if err != nil {
		return err
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

type vaultConfig struct {
	//Auth []xxx  `yaml:"auth"`
	Policies []policyItem `yaml:"policies"`
	//Secrets []xxx  `yaml:"secrets"`
	KV []kvItem `yaml:"kv"`
}

type policyItem struct {
	Name  string `yaml:"name"`
	Rules string `yaml:"rules"`
}

// Config contains data to write to Vault
// kv:
// - path: secret/data/infra/iitech/exdns
//   type: kv
//   data:
//     data:
//       CLIENT_ID: {{ vault }}
type kvItem struct {
	Type string                 `yaml:"type"`
	Path string                 `yaml:"path"`
	Data map[string]interface{} `yaml:"data"`
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

// VaultConfigKV writes  KV items to vault.
func (x *Execute) vaultConfigKV(vault *api.Client, kv []kvItem) error {
	for _, item := range kv {
		switch item.Type {
		case "kv":
			var err error
			x.Log.V(4).Info("Vault write kv", "path", item.Path)
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
		default:
			return fmt.Errorf("expected type 'kv', got: %s", item.Type)
		}
	}

	return nil
}

//TODO create set_vault_test.go
