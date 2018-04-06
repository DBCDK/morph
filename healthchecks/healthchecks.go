package healthchecks

import (
	"crypto/tls"
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"net/http"
	"time"
)

func Perform(host nix.Host) (err error) {
	fmt.Printf("Running healthchecks on %s:\n", nix.GetHostname(host))

	notOk := host.HealthChecks

	for len(notOk) > 0 {
		stillNotOk := make([]nix.HealthCheck, 0)

		for _, healthCheck := range notOk {
			fmt.Printf("\t* %s.. ", healthCheck.Description)
			if err := runCheck(host, healthCheck); err != nil {
				fmt.Println("Failed:", err)
				stillNotOk = append(stillNotOk, healthCheck)
			} else {
				fmt.Println("OK")
			}
		}

		notOk = stillNotOk
		time.Sleep(1 * time.Second)
	}
	return nil
}

func runCheck(host nix.Host, healthCheck nix.HealthCheck) (err error) {
	hostname := nix.GetHostname(host)
	if healthCheck.Host != nil {
		hostname = *healthCheck.Host
	}

	transport := &http.Transport{}

	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: healthCheck.InsecureSSL}

	client := &http.Client{
		Timeout:   time.Duration(healthCheck.Timeout) * time.Second,
		Transport: transport,
	}

	url := fmt.Sprintf("%s://%s:%d%s", healthCheck.Scheme, hostname, healthCheck.Port, healthCheck.Path)
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
