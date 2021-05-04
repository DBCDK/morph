package healthchecks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
	"net/http"
	"strings"
	"time"
)

type Host interface {
	GetName() string
	GetTargetHost() string
	GetTargetUser() string
	GetHealthChecks() HealthChecks
}

type HealthChecks struct {
	Http []HttpHealthCheck
	Cmd  []CmdHealthCheck
}

type CmdHealthCheck struct {
	SshContext  *ssh.SSHContext
	Description string
	Cmd         []string
	Period      int
	Timeout     int
}

type HttpHealthCheck struct {
	Description string
	Headers     map[string]string
	Host        *string
	InsecureSSL bool
	Path        string
	Port        int
	Scheme      string
	Period      int
	Timeout     int
}

type HealthCheck interface {
	GetDescription() string
	GetPeriod() int
	Run(Host) error
}

func (healthCheck CmdHealthCheck) GetDescription() string {
	return healthCheck.Description
}

func (healthCheck CmdHealthCheck) GetPeriod() int {
	return healthCheck.Period
}

func (healthCheck CmdHealthCheck) Run(host Host) error {
	ctx, cancel := utils.ContextWithConditionalTimeout(context.TODO(), healthCheck.Timeout)
	defer cancel()

	cmd, err := healthCheck.SshContext.CmdContext(ctx, host, healthCheck.Cmd...)
	if err != nil {
		errorMessage := fmt.Sprintf("Health check error: %s", err.Error())
		return errors.New(errorMessage)
	}
	data, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		errorMessage := fmt.Sprintf("Health check error: Timeout after %ds", healthCheck.Timeout)
		return errors.New(errorMessage)
	}
	if err != nil {
		errorMessage := fmt.Sprintf("Health check error: %s", string(data))
		return errors.New(errorMessage)
	}

	return nil

}

func (healthCheck HttpHealthCheck) GetDescription() string {
	return healthCheck.Description
}

func (healthCheck HttpHealthCheck) GetPeriod() int {
	return healthCheck.Period
}

func (healthCheck HttpHealthCheck) Run(host Host) error {
	// use the hosts hostname if the healthCheck host is not set
	if healthCheck.Host == nil {
		replacementHostname := host.GetTargetHost()
		healthCheck.Host = &replacementHostname
	}

	// http.Client interprets a timeout of 0 as "no timeout", but we still have to avoid passing
	// a negative timeout to it
	if healthCheck.Timeout < 0 {
		healthCheck.Timeout = 0
	}

	transport := &http.Transport{}

	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: healthCheck.InsecureSSL}

	client := &http.Client{
		Timeout:   time.Duration(healthCheck.Timeout) * time.Second,
		Transport: transport,
	}

	url := fmt.Sprintf("%s://%s:%d%s", healthCheck.Scheme, *healthCheck.Host, healthCheck.Port, healthCheck.Path)
	req, err := http.NewRequest("GET", url, nil)

	for headerKey, headerValue := range healthCheck.Headers {
		if strings.ToLower(headerKey) == "host" {
			req.Host = headerValue
		} else {
			req.Header.Add(headerKey, headerValue)
		}
	}

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	} else {
		return errors.New(fmt.Sprintf("Got non 2xx status code (%s)", resp.Status))
	}
}
