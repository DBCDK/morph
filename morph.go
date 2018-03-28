package main

import (
	"fmt"
	"git-platform.dbc.dk/platform/morph/assets"
	"git-platform.dbc.dk/platform/morph/filter"
	"git-platform.dbc.dk/platform/morph/nix"
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
	deploy                 = app.Command("deploy", "Deploy machines")
	deployment             = deploy.Arg("deployment", "File containing the deployment exec expression").Required().File()
	switchAction           = deploy.Arg("switch-action", "Either of dry-activate|test|switch|boot").Required().Enum("dry-activate", "test", "switch", "boot")
	deployOn               = deploy.Flag("on", "Glob for selecting servers in the deployment").Default("*").String()
	deployEvery            = deploy.Flag("every", "Select every n hosts").Default("1").Int()
	deploySkip             = deploy.Flag("skip", "Skip first n hosts").Default("0").Int()
	deployLimit            = deploy.Flag("limit", "Select at most n hosts").Int()
	deployDryRun           = deploy.Flag("dry-run", "Don't deploy anything, just eval and print changes").Default("False").Bool()
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

	matchingHosts, err := filter.MatchHosts(hosts, *deployOn)
	if err != nil {
		panic(err)
	}

	filteredHosts := filter.FilterHosts(matchingHosts, *deploySkip, *deployEvery, *deployLimit)

	fmt.Printf("Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(hosts), len(hosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, hostname := range nix.GetHostnames(filteredHosts) {
		fmt.Printf("\t%3d: %s\n", index, hostname)
	}
	fmt.Println()

	resultPath, err := nix.BuildMachines(evalMachinesPath, *deployment, filteredHosts)
	if err != nil {
		panic(err)
	}

	fmt.Println(resultPath)

	for _, host := range filteredHosts {
		paths, err := nix.GetPathsToPush(host, resultPath)
		if err != nil {
			panic(err)
		}
		fmt.Println(paths)
	}

	if !*deployDryRun {

		fmt.Println("Executing '" + *switchAction + "' on matched hosts:")
		sudoPasswd := ""
		if *deployAskForSudoPasswd && *switchAction != "dry-activate" {
			fmt.Print("Please enter remote sudo password: ")
			bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				panic(err)
			}
			sudoPasswd = string(bytePassword)
			fmt.Println()
		}

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

	} else {
		fmt.Println("Keeping it dry, aborting before connecting to any hosts ...")
	}

}
