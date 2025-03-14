package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DBCDK/kingpin"
	"github.com/DBCDK/morph/cliparser"
	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/filter"
	"github.com/DBCDK/morph/healthchecks"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/secrets"
	"github.com/DBCDK/morph/ssh"
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
		_, err := execEval(opts)
		handleError(err)
		return
	}

	// setup hosts
	hosts, err := getHosts(opts)
	handleError(err)

	switch clause {
	case cmdClauses.Build.FullCommand():
		_, err = execBuild(opts, hosts)
	case cmdClauses.Push.FullCommand():
		_, err = execPush(opts, hosts)
	case cmdClauses.Deploy.FullCommand():
		_, err = execDeploy(opts, hosts)
	case cmdClauses.HealthCheck.FullCommand():
		err = execHealthCheck(opts, hosts)
	case cmdClauses.SecretsUpload.FullCommand():
		err = execUploadSecrets(opts, hosts, nil)
	case cmdClauses.SecretsList.FullCommand():
		if opts.AsJson {
			err = execListSecretsAsJson(opts, hosts)
		} else {
			execListSecrets(hosts)
		}
	case cmdClauses.Execute.FullCommand():
		err = execExecute(opts, hosts)
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

func execExecute(opts *common.MorphOptions, hosts []nix.Host) error {
	sshContext := createSSHContext(opts)

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Exec is disabled for build-only host: %s\n", host.Name)
			continue
		}
		fmt.Fprintln(os.Stderr, "** "+host.Name)
		sshContext.CmdInteractive(&host, opts.Timeout, opts.ExecuteCommand...)
		fmt.Fprintln(os.Stderr)
	}

	return nil
}

func execBuild(opts *common.MorphOptions, hosts []nix.Host) (string, error) {
	resultPath, err := buildHosts(opts, hosts)
	if err != nil {
		return "", err
	}
	return resultPath, nil
}

func execEval(opts *common.MorphOptions) (string, error) {
	ctx := getNixContext(opts)

	deploymentFile, err := os.Open(opts.Deployment)
	deploymentPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return "", err
	}

	path, err := ctx.EvalHosts(deploymentPath, opts.AttrKey)

	return path, err
}

func execPush(opts *common.MorphOptions, hosts []nix.Host) (string, error) {
	resultPath, err := execBuild(opts, hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)
	return resultPath, pushPaths(createSSHContext(opts), hosts, resultPath)
}

func execDeploy(opts *common.MorphOptions, hosts []nix.Host) (string, error) {
	doPush := false
	doUploadSecrets := false
	doActivate := false

	if !*opts.DryRun {
		switch opts.DeploySwitchAction {
		case "dry-activate":
			doPush = true
			doActivate = true
		case "test":
			fallthrough
		case "switch":
			fallthrough
		case "boot":
			doPush = true
			doUploadSecrets = opts.DeployUploadSecrets
			doActivate = true
		}
	}

	resultPath, err := buildHosts(opts, hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)

	sshContext := createSSHContext(opts)

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Deployment steps are disabled for build-only host: %s\n", host.Name)
			continue
		}

		singleHostInList := []nix.Host{host}

		if doPush {
			err = pushPaths(sshContext, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}
		fmt.Fprintln(os.Stderr)

		if doUploadSecrets {
			phase := "pre-activation"
			err = execUploadSecrets(opts, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !opts.SkipPreDeployChecks {
			err := healthchecks.PerformPreDeployChecks(sshContext, &host, opts.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host pre-deploy check failed.")
				utils.Exit(1)
			}
		}

		if doActivate {
			err = activateConfiguration(opts, sshContext, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}

		if opts.DeployReboot {
			err = host.Reboot(sshContext)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Reboot failed")
				return "", err
			}
		}

		if doUploadSecrets {
			phase := "post-activation"
			err = execUploadSecrets(opts, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !opts.SkipHealthChecks {
			err := healthchecks.PerformHealthChecks(sshContext, &host, opts.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host health check failed.")
				utils.Exit(1)
			}
		}

		fmt.Fprintln(os.Stderr, "Done:", host.Name)
	}

	return resultPath, nil
}

func createSSHContext(opts *common.MorphOptions) *ssh.SSHContext {
	return &ssh.SSHContext{
		AskForSudoPassword:     opts.AskForSudoPasswd,
		GetSudoPasswordCommand: opts.PassCmd,
		IdentityFile:           os.Getenv("SSH_IDENTITY_FILE"),
		DefaultUsername:        os.Getenv("SSH_USER"),
		SkipHostKeyCheck:       os.Getenv("SSH_SKIP_HOST_KEY_CHECK") != "",
		ConfigFile:             os.Getenv("SSH_CONFIG_FILE"),
	}
}

func execHealthCheck(opts *common.MorphOptions, hosts []nix.Host) error {
	sshContext := createSSHContext(opts)

	var err error
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Healthchecks are disabled for build-only host: %s\n", host.Name)
			continue
		}
		err = healthchecks.PerformHealthChecks(sshContext, &host, opts.Timeout)
	}

	if err != nil {
		err = errors.New("One or more errors occurred during host healthchecks")
	}

	return err
}

func execUploadSecrets(opts *common.MorphOptions, hosts []nix.Host, phase *string) error {
	sshContext := createSSHContext(opts)

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Secret upload is disabled for build-only host: %s\n", host.Name)
			continue
		}
		singleHostInList := []nix.Host{host}

		err := secretsUpload(opts, sshContext, singleHostInList, phase)
		if err != nil {
			return err
		}

		if !opts.SkipHealthChecks {
			err = healthchecks.PerformHealthChecks(sshContext, &host, opts.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not uploading to additional hosts, since a host health check failed.")
				return err
			}
		}
	}

	return nil
}

func execListSecrets(hosts []nix.Host) {
	for _, host := range hosts {
		singleHostInList := []nix.Host{host}
		for _, host := range singleHostInList {
			fmt.Fprintf(os.Stdout, "Secrets for host %s:\n", host.Name)
			for name, secret := range host.Secrets {
				fmt.Fprintf(os.Stdout, "%s:\n- %v\n", name, &secret)
			}
			fmt.Fprintf(os.Stdout, "\n")
		}
	}
}

func execListSecretsAsJson(opts *common.MorphOptions, hosts []nix.Host) error {
	deploymentDir, err := filepath.Abs(filepath.Dir(opts.Deployment))
	if err != nil {
		return err
	}
	secretsByHost := make(map[string](map[string]secrets.Secret))

	for _, host := range hosts {
		singleHostInList := []nix.Host{host}
		for _, host := range singleHostInList {
			canonicalSecrets := make(map[string]secrets.Secret)
			for name, secret := range host.Secrets {
				sourcePath := utils.GetAbsPathRelativeTo(secret.Source, deploymentDir)
				secret.Source = sourcePath
				canonicalSecrets[name] = secret
			}
			secretsByHost[host.Name] = canonicalSecrets
		}
	}

	jsonSecrets, err := json.MarshalIndent(secretsByHost, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", jsonSecrets)

	return nil
}

func getHosts(opts *common.MorphOptions) (hosts []nix.Host, err error) {

	deploymentFile, err := os.Open(opts.Deployment)
	if err != nil {
		return hosts, err
	}

	deploymentAbsPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return hosts, err
	}

	ctx := getNixContext(opts)
	deployment, err := ctx.GetMachines(deploymentAbsPath)
	if err != nil {
		return hosts, err
	}

	matchingHosts, err := filter.MatchHosts(deployment.Hosts, opts.SelectGlob)
	if err != nil {
		return hosts, err
	}

	var selectedTags []string
	if opts.SelectTags != "" {
		selectedTags = strings.Split(opts.SelectTags, ",")
	}

	matchingHosts2 := filter.FilterHostsTags(matchingHosts, selectedTags)

	ordering := deployment.Meta.Ordering
	if opts.OrderingTags != "" {
		ordering = nix.HostOrdering{Tags: strings.Split(opts.OrderingTags, ",")}
	}

	sortedHosts := filter.SortHosts(matchingHosts2, ordering)

	filteredHosts := filter.FilterHosts(sortedHosts, opts.SelectSkip, opts.SelectEvery, opts.SelectLimit)

	fmt.Fprintf(os.Stderr, "Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(deployment.Hosts), len(deployment.Hosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "\t%3d: %s (secrets: %d, health checks: %d, tags: %s)\n", index, host.Name, len(host.Secrets), len(host.HealthChecks.Cmd)+len(host.HealthChecks.Http), strings.Join(host.GetTags(), ","))
	}
	fmt.Fprintln(os.Stderr)

	return filteredHosts, nil
}

func getNixContext(opts *common.MorphOptions) *nix.NixContext {
	evalCmd := os.Getenv("MORPH_NIX_EVAL_CMD")
	buildCmd := os.Getenv("MORPH_NIX_BUILD_CMD")
	shellCmd := os.Getenv("MORPH_NIX_SHELL_CMD")
	evalMachines := os.Getenv("MORPH_NIX_EVAL_MACHINES")

	if evalCmd == "" {
		evalCmd = "nix-instantiate"
	}
	if buildCmd == "" {
		buildCmd = "nix-build"
	}
	if shellCmd == "" {
		shellCmd = "nix-shell"
	}
	if evalMachines == "" {
		evalMachines = filepath.Join(opts.AssetRoot, "eval-machines.nix")
	}

	return &nix.NixContext{
		EvalCmd:         evalCmd,
		BuildCmd:        buildCmd,
		ShellCmd:        shellCmd,
		EvalMachines:    evalMachines,
		ShowTrace:       opts.ShowTrace,
		KeepGCRoot:      *opts.KeepGCRoot,
		AllowBuildShell: *opts.AllowBuildShell,
	}
}

func buildHosts(opts *common.MorphOptions, hosts []nix.Host) (resultPath string, err error) {
	if len(hosts) == 0 {
		err = errors.New("No hosts selected")
		return
	}

	deploymentPath, err := filepath.Abs(opts.Deployment)
	if err != nil {
		return
	}

	nixBuildTargets := ""
	if opts.NixBuildTargetFile != "" {
		if path, err := filepath.Abs(opts.NixBuildTargetFile); err == nil {
			nixBuildTargets = fmt.Sprintf("import \"%s\"", path)
		}
	} else if opts.NixBuildTarget != "" {
		nixBuildTargets = fmt.Sprintf("{ \"out\" = %s; }", opts.NixBuildTarget)
	}

	ctx := getNixContext(opts)
	resultPath, err = ctx.BuildMachines(deploymentPath, hosts, nixBuildTargets)

	if err != nil {
		return
	}

	fmt.Fprintln(os.Stderr, "nix result path: ")
	fmt.Println(resultPath)
	return
}

func pushPaths(sshContext *ssh.SSHContext, filteredHosts []nix.Host, resultPath string) error {
	for _, host := range filteredHosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Push is disabled for build-only host: %s\n", host.Name)
			continue
		}

		paths, err := nix.GetPathsToPush(host, resultPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Pushing paths to %v (%v@%v):\n", host.Name, host.TargetUser, host.TargetHost)
		for _, path := range paths {
			fmt.Fprintf(os.Stderr, "\t* %s\n", path)
		}
		err = nix.Push(sshContext, host, paths...)
		if err != nil {
			return err
		}
	}

	return nil
}

func secretsUpload(opts *common.MorphOptions, ctx ssh.Context, filteredHosts []nix.Host, phase *string) error {
	// upload secrets
	// relative paths are resolved relative to the deployment file (!)
	deploymentDir := filepath.Dir(opts.Deployment)
	for _, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "Uploading secrets to %s (%s):\n", host.Name, host.TargetHost)
		postUploadActions := make(map[string][]string, 0)
		for secretName, secret := range host.Secrets {
			// if phase is nil, upload the secrets no matter what phase it wants
			// if phase is non-nil, upload the secrets that match the specified phase
			if phase != nil && secret.UploadAt != *phase {
				continue
			}

			secretSize, err := secrets.GetSecretSize(secret, deploymentDir)
			if err != nil {
				return err
			}

			secretErr := secrets.UploadSecret(ctx, &host, secret, deploymentDir)
			fmt.Fprintf(os.Stderr, "\t* %s (%d bytes).. ", secretName, secretSize)
			if secretErr != nil {
				if secretErr.Fatal {
					fmt.Fprintln(os.Stderr, "Failed")
					return secretErr
				} else {
					fmt.Fprintln(os.Stderr, "Partial")
					fmt.Fprint(os.Stderr, secretErr.Error())
				}
			} else {
				fmt.Fprintln(os.Stderr, "OK")
			}
			if len(secret.Action) > 0 {
				// ensure each action is only run once
				postUploadActions[strings.Join(secret.Action, " ")] = secret.Action
			}
		}
		// Execute post-upload secret actions one-by-one after all secrets have been uploaded
		for _, action := range postUploadActions {
			fmt.Fprintf(os.Stderr, "\t- executing post-upload command: "+strings.Join(action, " ")+"\n")
			// Errors from secret actions will be printed on screen, but we won't stop the flow if they fail
			ctx.CmdInteractive(&host, opts.Timeout, action...)
		}
	}

	return nil
}

func activateConfiguration(opts *common.MorphOptions, ctx ssh.Context, filteredHosts []nix.Host, resultPath string) error {
	fmt.Fprintln(os.Stderr, "Executing '"+opts.DeploySwitchAction+"' on matched hosts:")
	fmt.Fprintln(os.Stderr)
	for _, host := range filteredHosts {

		fmt.Fprintln(os.Stderr, "** "+host.Name)

		configuration, err := nix.GetNixSystemPath(host, resultPath)
		if err != nil {
			return err
		}

		err = ctx.ActivateConfiguration(&host, configuration, opts.DeploySwitchAction)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr)
	}

	return nil
}
