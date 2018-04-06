package main

import (
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/assets"
	"git-platform.dbc.dk/platform/morph/filter"
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
	deployment             = deploy.Arg("deployment", "File containing the deployment exec expression").Required().File()
	switchAction           = deploy.Arg("switch-action", "Either of build|push|dry-activate|test|switch|boot").Required().Enum("build", "push", "dry-activate", "test", "switch", "boot")
	deployAskForSudoPasswd = deploy.Flag("passwd", "Whether to ask interactively for remote sudo password").Default("False").Bool()

	tempDir, tempDirErr = ioutil.TempDir("", "morph-")
)

var doPush = false
var doAskPass = false
var doUploadSecrets = false
var doActivate = false

func init() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	if tempDirErr != nil {
		panic(tempDirErr)
	}

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
}

func main() {
	if err := validateEnvironment(); err != nil {
		panic(err)
	}

	filteredHosts, resultPath := build()
	fmt.Println()

	if doPush {
		pushPaths(filteredHosts, resultPath)
	}
	fmt.Println()

	sudoPasswd := ""
	if doAskPass {
		sudoPasswd = askForSudoPassword()
		fmt.Println()
		fmt.Println()
	}

	if doUploadSecrets {
		uploadSecrets(filteredHosts, sudoPasswd)
	}

	if doActivate {
		activateConfiguration(filteredHosts, resultPath, sudoPasswd)
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

func build() ([]nix.Host, string) {
	// setup assets
	assetRoot, err := assets.Setup()
	if err != nil {
		panic(err)
	}
	defer assets.Teardown(assetRoot)

	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")
	// assets done
	deploymentPath, err := filepath.Abs((*deployment).Name())
	if err != nil {
		panic(err)
	}

	hosts, err := nix.GetMachines(evalMachinesPath, deploymentPath)
	if err != nil {
		panic(err)
	}

	matchingHosts, err := filter.MatchHosts(hosts, *selectGlob)
	if err != nil {
		panic(err)
	}

	filteredHosts := filter.FilterHosts(matchingHosts, *selectSkip, *selectEvery, *selectLimit)

	fmt.Printf("Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(hosts), len(hosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, host := range filteredHosts {
		fmt.Printf("\t%3d: %s (secrets: %d)\n", index, nix.GetHostname(host), len(host.Secrets))
	}
	fmt.Println()

	resultPath, err := nix.BuildMachines(evalMachinesPath, deploymentPath, filteredHosts)
	if err != nil {
		panic(err)
	}

	fmt.Println("nix result path: " + resultPath)
	return filteredHosts, resultPath
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
	deploymentDir := filepath.Dir((*deployment).Name())
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
	}
}
