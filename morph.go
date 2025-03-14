package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/DBCDK/kingpin"
	"github.com/DBCDK/morph/cliparser"
	"github.com/DBCDK/morph/cruft"
	"github.com/DBCDK/morph/utils"
)

// These are set at build time via -ldflags magic
var version string
var assetRoot string

func setup() {
	utils.ValidateEnvironment("nix")

	utils.SignalHandler()

	if assetRoot == "" {
		handleError(errors.New("Morph must be compiled with \"-ldflags=-X main.assetRoot=<path-to-installed-data/>\"."))
	}
}

func main() {

	cli, cmdClauses, opts := cliparser.New(version, assetRoot)
	clause := kingpin.MustParse(cli.Parse(os.Args[1:]))

	defer utils.RunFinalizers()
	setup()

	// evaluate without building hosts
	switch clause {
	case cmdClauses.Eval.FullCommand():
		_, err := cruft.ExecEval(opts)
		handleError(err)
		return
	}

	// setup hosts
	hosts, err := cruft.GetHosts(opts)
	handleError(err)

	switch clause {
	case cmdClauses.Build.FullCommand():
		_, err = cruft.ExecBuild(opts, hosts)
	case cmdClauses.Push.FullCommand():
		_, err = cruft.ExecPush(opts, hosts)
	case cmdClauses.Deploy.FullCommand():
		_, err = cruft.ExecDeploy(opts, hosts)
	case cmdClauses.HealthCheck.FullCommand():
		err = cruft.ExecHealthCheck(opts, hosts)
	case cmdClauses.SecretsUpload.FullCommand():
		err = cruft.ExecUploadSecrets(opts, hosts, nil)
	case cmdClauses.SecretsList.FullCommand():
		if opts.AsJson {
			err = cruft.ExecListSecretsAsJson(opts, hosts)
		} else {
			cruft.ExecListSecrets(hosts)
		}
	case cmdClauses.Execute.FullCommand():
		err = cruft.ExecExecute(opts, hosts)
	}

	handleError(err)
}

func handleError(err error) {
	//Stupid handling of catch-all errors for now
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		utils.Exit(1)
	}
}
