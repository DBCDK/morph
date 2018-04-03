package main

import (
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
	"path/filepath"
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

func init() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	if tempDirErr != nil {
		panic(tempDirErr)
	}
}

func main() {
	// setup assets
	assetRoot, err := assets.Setup()
	if err != nil {
		panic(err)
	}
	defer assets.Teardown(assetRoot)

	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")
	// assets done

	hosts, err := nix.GetMachines(evalMachinesPath, *deployment)
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

	resultPath, err := nix.BuildMachines(evalMachinesPath, *deployment, filteredHosts)
	if err != nil {
		panic(err)
	}

	fmt.Println("nix result path: " + resultPath)
	fmt.Println()

	if *switchAction == "build" {
		return
	}

	if *dryRun {
		fmt.Println("Keeping it dry, aborting before connecting to any hosts ...")
		return
	}

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

	fmt.Println()

	if *switchAction == "push" {
		return
	}

	sudoPasswd := ""
	if *deployAskForSudoPasswd && *switchAction != "dry-activate" {
		fmt.Print("Please enter remote sudo password: ")
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			panic(err)
		}
		sudoPasswd = string(bytePassword)
		fmt.Println()
		fmt.Println()
	}

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
