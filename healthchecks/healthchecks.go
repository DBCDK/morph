package healthchecks

import (
	"crypto/tls"
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"net/http"
	"sync"
	"time"
)

func Perform(host nix.Host, timeout *int) (err error) {
	fmt.Printf("Running healthchecks on %s:\n", nix.GetHostname(host))

	wg := sync.WaitGroup{}
	for _, healthCheck := range host.HealthChecks {
		// use the hosts hostname if the healthCheck host is not set
		if healthCheck.Host == nil {
			replacementHostname := nix.GetHostname(host)
			healthCheck.Host = &replacementHostname
		}
		wg.Add(1)
		go runCheckUntilSuccess(healthCheck, &wg)
	}

	doneChan := make(chan bool)

	go func() {
		wg.Wait()
		doneChan <- true
	}()

	// send timeout signal eventually
	timeoutChan := make(chan bool)
	if timeout != nil {
		go func() {
			time.Sleep(time.Duration(*timeout) * time.Second)
			timeoutChan <- true
		}()
	}

	// run health checks until done or timeout reached. Failing health checks will add themself to the chan again
	done := false
	for !done {
		select {
		case <-doneChan:
			fmt.Println("Health checks OK")
			done = true
		case <-timeoutChan:
			fmt.Printf("Timeout: Gave up waiting for health checks to complete after %d seconds\n", *timeout)
			return errors.New("timeout running health checks")
		}
	}

	return nil
}

func runCheckUntilSuccess(healthCheck nix.HealthCheck, wg *sync.WaitGroup) {
	for {
		err := runCheck(healthCheck)
		if err == nil {
			fmt.Printf("\t* %s: OK\n", healthCheck.Description)
			break
		} else {
			fmt.Printf("\t* %s: Failed (%s)\n", healthCheck.Description, err)
			time.Sleep(time.Duration(healthCheck.Period) * time.Second)
		}
	}
	wg.Done()
}

func runCheck(healthCheck nix.HealthCheck) (err error) {
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
