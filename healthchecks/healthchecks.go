package healthchecks

import (
	"errors"
	"fmt"
	"github.com/DBCDK/morph/ssh"
	"os"
	"sync"
	"time"
)

func PerformChecks(sshContext *ssh.SSHContext, checkName string, host Host, healthChecks HealthChecks, timeout int) (err error) {
	fmt.Fprintf(os.Stderr, "Running %s on %s (%s):\n", checkName, host.GetName(), host.GetTargetHost())

	wg := sync.WaitGroup{}
	for _, healthCheck := range healthChecks.Cmd {
		wg.Add(1)
		healthCheck.SshContext = sshContext
		go runCheckUntilSuccess(host, healthCheck, &wg)
	}
	for _, healthCheck := range healthChecks.Http {
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

	// run checks until done or timeout reached. Failing health checks will add themself to the chan again
	done := false
	for !done {
		select {
		case <-doneChan:
			fmt.Fprintln(os.Stderr, checkName+" OK")
			done = true
		case <-timeoutChan:
			fmt.Fprintf(os.Stderr, "Timeout: Gave up waiting for %s to complete after %d seconds\n", checkName, timeout)
			return errors.New(fmt.Sprintf("timeout running %s on %s", checkName, host.GetName()))
		}
	}

	return nil
}

func PerformPreDeployChecks(sshContext *ssh.SSHContext, host Host, timeout int) (err error) {
	return PerformChecks(sshContext, "pre-deploy checks", host, host.GetPreDeployChecks(), timeout)
}

func PerformHealthChecks(sshContext *ssh.SSHContext, host Host, timeout int) (err error) {
	return PerformChecks(sshContext, "health checks", host, host.GetHealthChecks(), timeout)
}

func runCheckUntilSuccess(host Host, healthCheck HealthCheck, wg *sync.WaitGroup) {
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
