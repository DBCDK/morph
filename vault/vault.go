package vault

import (
	"crypto/tls"
	"git-platform.dbc.dk/platform/morph/nix"
	vault "github.com/hashicorp/vault/api"
	"net/http"
	"strings"
	"time"
)

func Auth(addr string, rootToken string) (vc *vault.Client, err error) {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}

	config := vault.Config{
		Address:    addr,
		HttpClient: &http.Client{Transport: tr},
		Timeout:    5 * time.Second,
	}

	vc, err = vault.NewClient(&config)
	if err != nil {
		return nil, err
	}

	vc.SetToken(rootToken)
	vc.Auth()

	return vc, nil
}

/*
	Configure func is needed because we want Vault config to behave in a declarative manner
	Most likely, these features will be enabled
*/
func Configure(vc *vault.Client) error {

	//auths, _ := vc.Sys().ListAuth()
	audits, _ := vc.Sys().ListAudit()

	//authAppRole := false
	//for a := range auths {
	//	if a == "approle/" {
	//		authAppRole = true
	//	}
	//}

	auditSysLog := false
	for a := range audits {
		if a == "syslog/" {
			auditSysLog = true
		}
	}

	if false { // disable app-role auth for now
		err := vc.Sys().EnableAuthWithOptions("approle", &vault.EnableAuthOptions{
			Type:        "approle",
			Description: "Enable auth approle",
		})
		if err != nil && !strings.Contains(err.Error(), "already in use") {
			return err
		}
	}

	if !auditSysLog {
		err := vc.Sys().EnableAuditWithOptions("syslog", &vault.EnableAuditOptions{
			Type:        "syslog",
			Description: "Enable audit syslog",
		})
		if err != nil && !strings.Contains(err.Error(), "already in use") {
			return err
		}
	}

	return nil
}

func CreateOrReKeyHostToken(vc *vault.Client, host nix.Host) (*TokenCredentials, error) {

	err := syncTokenRole(vc, host)
	if err != nil {
		return nil, err
	}

	secret, err := newToken(vc, host)
	if err != nil {
		return nil, err
	}

	return &TokenCredentials{
		Accessor: secret.Auth.Accessor,
		Token:    secret.Auth.ClientToken}, nil
}

type AppRoleCredentials struct {
	RoleID   string
	SecretID string
}

type TokenCredentials struct {
	Accessor string
	Token    string
}
