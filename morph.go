package main

import (
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/assets"
	"git-platform.dbc.dk/platform/morph/filter"
	"git-platform.dbc.dk/platform/morph/healthchecks"
	"git-platform.dbc.dk/platform/morph/nix"
	"git-platform.dbc.dk/platform/morph/secrets"
	"git-platform.dbc.dk/platform/morph/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	app                    = kingpin.New("morph", "NixOS host manager").Version("1.0")
	dryRun                 = app.Flag("dry-run", "Don't do anything, just eval and print changes").Default("False").Bool()
	selectGlob             = app.Flag("on", "Glob for selecting servers in the deployment").Default("*").String()
	selectEvery            = app.Flag("every", "Select every n hosts").Default("1").Int()
	selectSkip             = app.Flag("skip", "Skip first n hosts").Default("0").Int()
	selectLimit            = app.Flag("limit", "Select at most n hosts").Int()
	deploy                 = app.Command("deploy", "Deploy machines")
	deployDeployment       = deploy.Arg("deployment", "File containing the deployment exec expression").Required().File()
	switchAction           = deploy.Arg("switch-action", "Either of build|push|dry-activate|test|switch|boot").Required().Enum("build", "push", "dry-activate", "test", "switch", "boot")
	deployAskForSudoPasswd = deploy.Flag("passwd", "Whether to ask interactively for remote sudo password").Default("False").Bool()
	deploySkipHealthChecks = deploy.Flag("skip-health-checks", "Whether to ask interactively for remote sudo password").Default("False").Bool()
	deployHealthCheckTimeout     = deploy.Flag("health-check-timeout", "Seconds to wait for all health checks on a host to complete").Int()
	healthCheck            = app.Command("check-health", "Run health checks")
	healthCheckDeployment  = healthCheck.Arg("deployment", "File containing the deployment exec expression").Required().File()
	healthCheckTimeout     = healthCheck.Flag("timeout", "Seconds to wait for all health checks on a host to complete").Int()

	tempDir, tempDirErr  = ioutil.TempDir("", "morph-")
	assetRoot, assetsErr = assets.Setup()
)

var doPush = false
var doAskPass = false
var doUploadSecrets = false
var doActivate = false

func init() {
	if err := validateEnvironment(); err != nil {
		panic(err)
	}

	if assetsErr != nil {
		fmt.Println("Error unpacking assets:")
		panic(assetsErr)
	}

	if tempDirErr != nil {
		panic(tempDirErr)
	}
}

func main() {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case deploy.FullCommand():
		doDeploy()
	case healthCheck.FullCommand():
		doHealthCheck()
	}

	assets.Teardown(assetRoot)
}

func doDeploy() {
	if !*dryRun {
		switch *switchAction {
		case "push":
			doPush = true
		case "dry-activate":
			doPush = true
			// fixme (in ssh/ssh.go) - should be possible to dry-activate without sudo
			if *deployAskForSudoPasswd {
				doAskPass = true
			}
			doActivate = true
		case "test":
			fallthrough
		case "switch":
			fallthrough
		case "boot":
			doPush = true
			if *deployAskForSudoPasswd {
				doAskPass = true
			}
			doUploadSecrets = true
			doActivate = true
		}
	}

	hosts, resultPath := build()
	fmt.Println()

	if doPush {
		pushPaths(hosts, resultPath)
	}
	fmt.Println()

	sudoPasswd := ""
	if doAskPass {
		sudoPasswd = askForSudoPassword()
		fmt.Println()
		fmt.Println()
	}

	if doUploadSecrets {
		uploadSecrets(hosts, sudoPasswd)
	}

	if doActivate {
		activateConfiguration(hosts, resultPath, sudoPasswd)
	}

}

func doHealthCheck() {
	hosts, err := getHosts(*healthCheckDeployment)
	if err != nil {
		panic(err)
	}

	for _, host := range hosts {
		healthchecks.Perform(host, healthCheckTimeout)
	}
}

func validateEnvironment() (err error) {
	dependencies := []string{"nix", "scp", "ssh"}
	missingDepencies := make([]string, 0)
	for _, dependency := range dependencies {
		_, err := exec.LookPath(dependency)
		if err != nil {
			missingDepencies = append(missingDepencies, dependency)
		}
	}

	if len(missingDepencies) > 0 {
		return errors.New("Missing dependencies: " + strings.Join(missingDepencies, ", "))
	}

	return nil
}

func getHosts(deployment *os.File) (hosts []nix.Host, err error) {
	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")

	deploymentPath, err := filepath.Abs(deployment.Name())
	if err != nil {
		return hosts, err
	}

	allHosts, err := nix.GetMachines(evalMachinesPath, deploymentPath)
	if err != nil {
		return hosts, err
	}

	matchingHosts, err := filter.MatchHosts(allHosts, *selectGlob)
	if err != nil {
		return hosts, err
	}

	filteredHosts := filter.FilterHosts(matchingHosts, *selectSkip, *selectEvery, *selectLimit)

	fmt.Printf("Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(allHosts), len(allHosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, host := range filteredHosts {
		fmt.Printf("\t%3d: %s (secrets: %d, health checks: %d)\n", index, nix.GetHostname(host), len(host.Secrets), len(host.HealthChecks))
	}
	fmt.Println()

	return filteredHosts, nil
}

func build() ([]nix.Host, string) {
	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")

	deploymentPath, err := filepath.Abs((*deployDeployment).Name())
	if err != nil {
		panic(err)
	}

	hosts, err := getHosts(*deployDeployment)
	if err != nil {
		panic(err)
	}

	resultPath, err := nix.BuildMachines(evalMachinesPath, deploymentPath, hosts)
	if err != nil {
		panic(err)
	}

	fmt.Println("nix result path: " + resultPath)
	return hosts, resultPath
}

func askForSudoPassword() string {
	fmt.Print("Please enter remote sudo password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		panic(err)
	}
	return string(bytePassword)
}

func pushPaths(filteredHosts []nix.Host, resultPath string) {
	for _, host := range filteredHosts {
		paths, err := nix.GetPathsToPush(host, resultPath)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Pushing paths to %v:\n", host.TargetHost)
		for _, path := range paths {
			fmt.Printf("\t* %s\n", path)
		}
		nix.Push(host, paths...)
	}
}

func uploadSecrets(filteredHosts []nix.Host, sudoPasswd string) {
	// upload secrets
	// relative paths are resolved relative to the deployment file (!)
	deploymentDir := filepath.Dir((*deployDeployment).Name())
	for _, host := range filteredHosts {
		fmt.Printf("Uploading secrets to %s:\n", nix.GetHostname(host))
		for secretName, secret := range host.Secrets {
			secretSize, err := secrets.GetSecretSize(secret, deploymentDir)
			if err != nil {
				panic(err)
			}

			fmt.Printf("\t* %s (%d bytes).. ", secretName, secretSize)
			err = secrets.UploadSecret(host, sudoPasswd, secret, deploymentDir)
			if err != nil {
				fmt.Println("Failed")
				panic(err)
			} else {
				fmt.Println("OK")
			}
		}
	}
}

func activateConfiguration(filteredHosts []nix.Host, resultPath string, sudoPasswd string) {
	fmt.Println("Executing '" + *switchAction + "' on matched hosts:")
	fmt.Println()
	for _, host := range filteredHosts {

		fmt.Println("** " + host.TargetHost)

		configuration, err := nix.GetNixSystemPath(host, resultPath)
		if err != nil {
			panic(err)
		}

		err = ssh.ActivateConfiguration(host, configuration, *switchAction, sudoPasswd)
		if err != nil {
			panic(err)
		}

		fmt.Println()

		if !*deploySkipHealthChecks {
			healthchecks.Perform(host, deployHealthCheckTimeout)
		}
	}
}
