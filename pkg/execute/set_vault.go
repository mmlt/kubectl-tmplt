package execute

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault/api"
	"github.com/mmlt/kubectl-tmplt/pkg/util/yamlx"
	"gopkg.in/yaml.v3"
	"strings"
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

	if portForward != "" {
		ctx, cancel := context.WithCancel(context.Background())
		x.kubectlPortForward(ctx, portForward)
		defer cancel()
	}

	vault, err := newVaultClient(av.URL, av.CA, av.TLSSkipVerify == "true", av.Token)
	if err != nil {
		return err
	}

	//err = vaultConfigPolicies(vault, av.Config.Policies)
	//if err != nil {
	//	return err
	//}

	err = vaultConfigKV(vault, av.Config.KV)
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

func (x *Execute) kubectlPortForward(ctx context.Context, flags string) {
	args := strings.Split(flags, " ")
	args = append([]string{"port-forward"}, args...)
	go func() {
		_, _, _ = x.Kubectl.Run(ctx, "", args...) // error is logged
	}()
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

	return clnt, nil
}

// VaultConfigKV writes kvItems to vault.
func vaultConfigKV(vault *api.Client, kv []kvItem) error {
	for _, item := range kv {
		switch item.Type {
		case "kv":
			//TODO retry with backoff
			_, err := vault.Logical().Write(item.Path, item.Data)
			if err != nil {
				return err //TODO add path to err?
			}
		default:
			return fmt.Errorf("expected type 'kv', got: %s", item.Type)
		}
	}

	return nil
}

//TODO create set_vault_test.go
