package healthchecks

import (
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"sync"
	"time"
)

func Perform(host nix.Host, timeout int) (err error) {
	fmt.Printf("Running healthchecks on %s:\n", nix.GetHostname(host))

	wg := sync.WaitGroup{}
	for _, healthCheck := range host.HealthChecks.Http {
		// use the hosts hostname if the healthCheck host is not set
		if healthCheck.Host == nil {
			replacementHostname := nix.GetHostname(host)
			healthCheck.Host = &replacementHostname
		}
		wg.Add(1)
		go httpRunCheckUntilSuccess(healthCheck, &wg)
	}

	doneChan := make(chan bool)

	go func() {
		wg.Wait()
		doneChan <- true
	}()

	// send timeout signal eventually
	timeoutChan := make(chan bool)
	if timeout > 0 {
		go func() {
			time.Sleep(time.Duration(timeout) * time.Second)
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
			fmt.Printf("Timeout: Gave up waiting for health checks to complete after %d seconds\n", timeout)
			return errors.New("timeout running health checks")
		}
	}

	return nil
}

func httpRunCheckUntilSuccess(healthCheck nix.HealthCheck, wg *sync.WaitGroup) {
	for {
		err := healthCheck.Run()
		if err == nil {
			fmt.Printf("\t* %s: OK\n", healthCheck.GetDescription())
			break
		} else {
			fmt.Printf("\t* %s: Failed (%s)\n", healthCheck.GetDescription(), err)
			time.Sleep(time.Duration(healthCheck.GetPeriod()) * time.Second)
		}
	}
	wg.Done()
}
