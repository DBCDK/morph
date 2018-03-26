package main

import (
	"fmt"
	filter "git-platform.dbc.dk/platform/morph/filter"
	nix "git-platform.dbc.dk/platform/morph/nix"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
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
)

func init() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func main() {
	fmt.Println((*deployment).Name())
	hosts, err := nix.GetMachines("/home/atu/go/src/git-platform.dbc.dk/platform/morph/data/eval-machines.nix", *deployment)
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

	resultPath, err := nix.BuildMachines("/home/atu/go/src/git-platform.dbc.dk/platform/morph/data/eval-machines.nix", *deployment, filteredHosts)
	if err != nil {
		panic(err)
	}

	fmt.Println(resultPath)
}
