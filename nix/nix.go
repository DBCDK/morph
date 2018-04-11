package nix

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Host struct {
	HealthChecks HealthChecks
	Name         string
	NixosRelease string
	TargetHost   string
	Secrets      map[string]Secret
	Vault        VaultOptions
}

type HealthChecks struct {
	Http []HttpHealthCheck
	Cmd  []CmdHealthCheck
}

type CmdHealthCheck struct {
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

type Secret struct {
	Source      string
	Destination string
	Owner       Owner
	Permissions string
}

type Owner struct {
	Group string
	User  string
}

type VaultOptions struct {
	CIDRs           []string
	Policies        []string
	TTL             string
	DestinationFile VaultDestinationFile
}

type VaultDestinationFile struct {
	Path        string
	Owner       Owner
	Permissions string
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
	args := []string{GetHostname(host)}
	args = append(args, healthCheck.Cmd...)

	cmd := exec.Command(
		"ssh", args...,
	)

	data, err := cmd.CombinedOutput()
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
		replacementHostname := GetHostname(host)
		healthCheck.Host = &replacementHostname
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
		req.Header.Add(headerKey, headerValue)
	}

	resp, err := client.Get(url)

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

func GetMachines(evalMachines string, deploymentPath string) (hosts []Host, err error) {
	cmd := exec.Command(
		"nix", "eval",
		"-f", evalMachines, "info.machineList",
		"--arg", "networkExpr", deploymentPath,
		"--json",
	)

	bytes, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err)
		errorMessage := fmt.Sprintf(
			"Error while running `nix eval ..`:\n%s", string(bytes),
		)
		return hosts, errors.New(errorMessage)
	}

	err = json.Unmarshal(bytes, &hosts)
	if err != nil {
		return hosts, err
	}

	return hosts, nil
}

func BuildMachines(evalMachines string, deploymentPath string, hosts []Host) (path string, err error) {
	hostsArg := "["
	for _, host := range hosts {
		hostsArg += "\"" + host.TargetHost + "\" "
	}
	hostsArg += "]"

	// create tmp dir for result link
	tmpdir, err := ioutil.TempDir("", "morph-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpdir)

	resultLinkPath := filepath.Join(tmpdir, "result")

	cmd := exec.Command(
		"nix", "build",
		"-f", evalMachines, "machines",
		"--arg", "networkExpr", deploymentPath,
		"--arg", "names", hostsArg,
		"--out-link", resultLinkPath,
	)
	defer os.Remove(resultLinkPath)

	// show process output on attached stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error while running `nix build ...`: See above.",
		)
		return path, errors.New(errorMessage)
	}

	resultPath, err := os.Readlink(resultLinkPath)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

func GetNixSystemPath(host Host, resultPath string) (string, error) {
	return os.Readlink(filepath.Join(resultPath, host.Name))
}

func GetNixSystemDerivation(host Host, resultPath string) (string, error) {
	return os.Readlink(filepath.Join(resultPath, host.Name+".drv"))
}

func GetPathsToPush(host Host, resultPath string) (paths []string, err error) {
	path1, err := GetNixSystemPath(host, resultPath)
	if err != nil {
		return paths, err
	}

	path2, err := GetNixSystemDerivation(host, resultPath)
	if err != nil {
		return paths, err
	}

	paths = append(paths, path1, path2)

	return paths, nil
}

func Push(host Host, paths ...string) (err error) {
	for _, path := range paths {
		cmd := exec.Command(
			"nix", "copy",
			path,
			"--to", "ssh://"+host.TargetHost,
		)

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()

		if err != nil {
			return err
		}
	}

	return nil
}

func GetHostname(host Host) string {
	return host.TargetHost
}

func GetHostnames(hosts []Host) (hostnames []string) {
	for _, host := range hosts {
		hostnames = append(hostnames, GetHostname(host))
	}

	return
}
