package healthchecks

import (
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"os"
	"sync"
	"time"
)

func Perform(host nix.Host, timeout int) (err error) {
	fmt.Fprintf(os.Stderr, "Running healthchecks on %s:\n", nix.GetHostname(host))

	wg := sync.WaitGroup{}
	for _, healthCheck := range host.HealthChecks.Cmd {
		wg.Add(1)
		go runCheckUntilSuccess(host, healthCheck, &wg)
	}
	for _, healthCheck := range host.HealthChecks.Http {
		wg.Add(1)
		go runCheckUntilSuccess(host, healthCheck, &wg)
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
			fmt.Fprintln(os.Stderr, "Health checks OK")
			done = true
		case <-timeoutChan:
			fmt.Fprintf(os.Stderr, "Timeout: Gave up waiting for health checks to complete after %d seconds\n", timeout)
			return errors.New("timeout running health checks")
		}
	}

	return nil
}

func runCheckUntilSuccess(host nix.Host, healthCheck nix.HealthCheck, wg *sync.WaitGroup) {
	for {
		err := healthCheck.Run(host)
		if err == nil {
			fmt.Fprintf(os.Stderr, "\t* %s: OK\n", healthCheck.GetDescription())
			break
		} else {
			fmt.Fprintf(os.Stderr, "\t* %s: Failed (%s)\n", healthCheck.GetDescription(), err)
			time.Sleep(time.Duration(healthCheck.GetPeriod()) * time.Second)
		}
	}
	wg.Done()
}
