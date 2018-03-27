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
	assetRoot, err := assets.Setup()
	if err != nil {panic(err)}
	defer assets.Teardown(assetRoot)

	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")
	fmt.Println(assetRoot)

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
