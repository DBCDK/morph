package main

import (
	"fmt"
	"git-platform.dbc.dk/platform/morph/assets"
	filter "git-platform.dbc.dk/platform/morph/filter"
	nix "git-platform.dbc.dk/platform/morph/nix"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	app          = kingpin.New("morph", "NixOS host manager").Version("1.0")
	deploy       = app.Command("deploy", "Deploy machines")
	deployment   = deploy.Arg("deployment", "File containing the deployment nix expression").Required().File()
	deployOn     = deploy.Flag("on", "Glob for selecting servers in the deployment").Default("*").String()
	deployEvery  = deploy.Flag("every", "Select every n hosts").Default("1").Int()
	deploySkip   = deploy.Flag("skip", "Skip first n hosts").Default("0").Int()
	deployLimit  = deploy.Flag("limit", "Select at most n hosts").Int()
	deployDryRun = deploy.Flag("dry-run", "Don't perform any actions").Default("False").Bool()

	tempDir, tempDirErr		 = ioutil.TempDir("", "morph-")
)

func init() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	if tempDirErr != nil {
		panic(tempDirErr)
	}
}

func main() {
	// setup assets

	tempDir, err := ioutil.TempDir("", "morph-")
	if err != nil {panic(err)}
	defer os.RemoveAll(tempDir)

	evalMachinesData, err := assets.Asset("data/eval-machines.nix")
	if err != nil {
		if err != nil {panic(err)}
	}

	optionsData, err := assets.Asset("data/options.nix")
	if err != nil {
		if err != nil {panic(err)}
	}

	evalMachinesPath := filepath.Join(tempDir, "eval-machines.nix")
	optionsPath := filepath.Join(tempDir, "options.nix")
	ioutil.WriteFile(evalMachinesPath, evalMachinesData, 0644)
	ioutil.WriteFile(optionsPath, optionsData, 0644)

	fmt.Println(tempDir)

	// assets done


	fmt.Println((*deployment).Name())
	hosts, err := nix.GetMachines(evalMachinesPath, *deployment)
	if err != nil {
		panic(err)
	}

	fmt.Println(hosts)
	matchingHosts, err := filter.MatchHosts(hosts, *deployOn)
	if err != nil {
		panic(err)
	}

	fmt.Println(matchingHosts)
	filteredHosts := filter.FilterHosts(matchingHosts, *deploySkip, *deployEvery, *deployLimit)
	fmt.Println(filteredHosts)

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
		for _, path := range paths {
			nix.Push(host, path)
		}
	}
}
