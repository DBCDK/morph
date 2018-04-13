package vault

import (
	"git-platform.dbc.dk/platform/morph/nix"
	vault "github.com/hashicorp/vault/api"
)

func syncTokenRole(client *vault.Client, host nix.Host) error {

	r := client.NewRequest("POST", "/v1/auth/token/roles/"+host.TargetHost)

	if err := r.SetJSONBody(roleCreateRequest{
		AllowedPolicies:    host.Vault.Policies,
		DisallowedPolicies: []string{"root"},
		Orphan:             true,
		Renewable:          true,
		MaxTTL:             host.Vault.TTL}); err != nil {
		return err
	}

	_, err := client.RawRequest(r)
	if err != nil {
		return err
	}

	return nil
}

func newToken(client *vault.Client, host nix.Host) (*vault.Secret, error) {

	r := client.NewRequest("POST", "/v1/auth/token/create/"+host.TargetHost)

	if err := r.SetJSONBody(tokenCreateRequest{
		Policies:        host.Vault.Policies,
		NoDefaultPolicy: true,
		DisplayName:     host.TargetHost}); err != nil {
		return nil, err
	}

	resp, err := client.RawRequest(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return vault.ParseSecret(resp.Body)
}

type roleCreateRequest struct {
	PathSuffix         string   `json:"path_suffix,omitempty"`
	AllowedPolicies    []string `json:"allowed_policies,omitempty"`
	DisallowedPolicies []string `json:"disallowed_policies,omitempty"`
	Orphan             bool     `json:"orphan,omitempty"`
	Renewable          bool     `json:"renewable,omitempty"`
	MaxTTL             string   `json:"explicit_max_ttl,omitempty"`
}

type tokenCreateRequest struct {
	Policies        []string          `json:"policies,omitempty"`
	Metadata        map[string]string `json:"meta,omitempty"`
	NoParent        bool              `json:"no_parent,omitempty"`
	NoDefaultPolicy bool              `json:"no_default_policy,omitempty"`
	MaxTTL          string            `json:"explicit_max_ttl,omitempty"`
	DisplayName     string            `json:"display_name,omitempty"`
}
