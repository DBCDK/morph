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
	"github.com/DBCDK/kingpin"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var switchActions = []string{"dry-activate", "test", "switch", "boot"}

var (
	app                    = kingpin.New("morph", "NixOS host manager").Version("1.0")
	dryRun                 = app.Flag("dry-run", "Don't do anything, just eval and print changes").Default("False").Bool()
	selectGlob             string
	selectEvery            int
	selectSkip             int
	selectLimit            int
	deployment             string
	timeout                int
	askForSudoPasswd       bool
	build                  = buildCmd(app.Command("build", "Build machines"))
	push                   = pushCmd(app.Command("push", "Push machines"))
	deploy                 = deployCmd(app.Command("deploy", "Deploy machines"))
	deploySwitchAction     string
	deploySkipHealthChecks bool
	healthCheck            = healthCheckCmd(app.Command("check-health", "Run health checks"))
	execute                = executeCmd(app.Command("exec", "Execute arbitrary commands on machines"))
	executeCommand         []string

	tempDir, tempDirErr  = ioutil.TempDir("", "morph-")
	assetRoot, assetsErr = assets.Setup()
)

var doPush = false
var doUploadSecrets = false
var doActivate = false

func deploymentArg(cmd *kingpin.CmdClause) {
	cmd.Arg("deployment", "File containing the nix deployment expression").
		HintFiles("nix").
		Required().
		ExistingFileVar(&deployment)
}

func timeoutFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("timeout", "Seconds to wait for commands/healthchecks on a host to complete").
		Default("0").
		IntVar(&timeout)
}

func askForSudoPasswdFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("passwd", "Whether to ask interactively for remote sudo password when needed").
		Default("False").
		BoolVar(&askForSudoPasswd)
}

func selectorFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("on", "Glob for selecting servers in the deployment").
		Default("*").
		StringVar(&selectGlob)
	cmd.Flag("every", "Select every n hosts").
		Default("1").
		IntVar(&selectEvery)
	cmd.Flag("skip", "Skip first n hosts").
		Default("0").
		IntVar(&selectSkip)
	cmd.Flag("limit", "Select at most n hosts").
		IntVar(&selectLimit)
}

func buildCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	deploymentArg(cmd)
	return cmd
}

func pushCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	deploymentArg(cmd)
	return cmd
}

func executeCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	askForSudoPasswdFlag(cmd)
	timeoutFlag(cmd)
	deploymentArg(cmd)
	cmd.
		Arg("command", "Command to execute").
		Required().
		StringsVar(&executeCommand)
	cmd.NoInterspersed = true
	return cmd
}

func deployCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	deploymentArg(cmd)
	timeoutFlag(cmd)
	askForSudoPasswdFlag(cmd)
	cmd.
		Flag("skip-health-checks", "Whether to skip all health checks").
		Default("False").
		BoolVar(&deploySkipHealthChecks)
	cmd.
		Arg("switch-action", "Either of "+strings.Join(switchActions, "|")).
		Required().
		HintOptions(switchActions...).
		EnumVar(&deploySwitchAction, switchActions...)
	return cmd
}

func healthCheckCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	deploymentArg(cmd)
	timeoutFlag(cmd)
	return cmd
}

func init() {
	if err := validateEnvironment(); err != nil {
		panic(err)
	}

	if assetsErr != nil {
		fmt.Fprintln(os.Stderr, "Error unpacking assets:")
		panic(assetsErr)
	}

	if tempDirErr != nil {
		panic(tempDirErr)
	}
}

func main() {

	clause := kingpin.MustParse(app.Parse(os.Args[1:]))

	hosts, err := getHosts(deployment)
	if err != nil {
		panic(err)
	}

	switch clause {
	case build.FullCommand():
		execBuild(hosts)
	case push.FullCommand():
		execPush(hosts)
	case deploy.FullCommand():
		execDeploy(hosts)
	case healthCheck.FullCommand():
		execHealthCheck(hosts)
	case execute.FullCommand():
		execExecute(hosts)
	}

	assets.Teardown(assetRoot)
}

func execExecute(hosts []nix.Host) {
	sshContext := ssh.SSHContext{
		AskForSudoPassword: askForSudoPasswd,
	}

	for _, host := range hosts {
		fmt.Fprintln(os.Stderr, "** " + host.Name)
		sshContext.CmdInteractive(host, timeout, executeCommand...)
		fmt.Fprintln(os.Stderr)
	}

}

func execBuild(hosts []nix.Host) (string, error) {
	resultPath, err := buildHosts(hosts)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

func execPush(hosts []nix.Host) {
	resultPath, err := execBuild(hosts)
	if err != nil {
		panic(err)
	}
	pushPaths(hosts, resultPath)
}

func execDeploy(hosts []nix.Host) {
	if !*dryRun {
		switch deploySwitchAction {
		case "dry-activate":
			doPush = true
			doActivate = true
		case "test":
			fallthrough
		case "switch":
			fallthrough
		case "boot":
			doPush = true
			doUploadSecrets = true
			doActivate = true
		}
	}

	resultPath, err := buildHosts(hosts)
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stderr)

	sshContext := ssh.SSHContext{
		AskForSudoPassword: askForSudoPasswd,
	}

	for _, host := range hosts {
		singleHostInList := []nix.Host{host}

		if doPush {
			pushPaths(singleHostInList, resultPath)
		}
		fmt.Fprintln(os.Stderr)

		if doUploadSecrets {
			uploadSecrets(&sshContext, singleHostInList)
		}

		if doActivate {
			activateConfiguration(&sshContext, singleHostInList, resultPath)
		}

		if !deploySkipHealthChecks {
			err := healthchecks.Perform(host, timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host health check failed.")
				os.Exit(1)
			}
		}

		fmt.Fprintln(os.Stderr, "Done:", nix.GetHostname(host))
	}
}

func execHealthCheck(hosts []nix.Host) {
	for _, host := range hosts {
		healthchecks.Perform(host, timeout)
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

func getHosts(deploymentFile string) (hosts []nix.Host, err error) {

	deployment, err := os.Open(deploymentFile)
	if err != nil {
		return hosts, err
	}

	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")

	deploymentPath, err := filepath.Abs(deployment.Name())
	if err != nil {
		return hosts, err
	}

	allHosts, err := nix.GetMachines(evalMachinesPath, deploymentPath)
	if err != nil {
		return hosts, err
	}

	matchingHosts, err := filter.MatchHosts(allHosts, selectGlob)
	if err != nil {
		return hosts, err
	}

	filteredHosts := filter.FilterHosts(matchingHosts, selectSkip, selectEvery, selectLimit)

	fmt.Fprintf(os.Stderr, "Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(allHosts), len(allHosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "\t%3d: %s (secrets: %d, health checks: %d)\n", index, nix.GetHostname(host), len(host.Secrets), len(host.HealthChecks.Cmd)+len(host.HealthChecks.Http))
	}
	fmt.Fprintln(os.Stderr)

	return filteredHosts, nil
}

func buildHosts(hosts []nix.Host) (resultPath string, err error) {
	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")

	if len(hosts) == 0 {
		err = errors.New("No hosts selected")
		return
	}

	deploymentPath, err := filepath.Abs(deployment)
	if err != nil {
		return
	}

	resultPath, err = nix.BuildMachines(evalMachinesPath, deploymentPath, hosts)
	if err != nil {
		return
	}

	fmt.Fprintln(os.Stderr, "nix result path: ")
	fmt.Println(resultPath)
	return
}

func pushPaths(filteredHosts []nix.Host, resultPath string) {
	for _, host := range filteredHosts {
		paths, err := nix.GetPathsToPush(host, resultPath)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(os.Stderr, "Pushing paths to %v:\n", host.TargetHost)
		for _, path := range paths {
			fmt.Fprintf(os.Stderr, "\t* %s\n", path)
		}
		nix.Push(host, paths...)
	}
}

func uploadSecrets(ctx ssh.Context, filteredHosts []nix.Host) {
	// upload secrets
	// relative paths are resolved relative to the deployment file (!)
	deploymentDir := filepath.Dir(deployment)
	for _, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "Uploading secrets to %s:\n", nix.GetHostname(host))
		for secretName, secret := range host.Secrets {
			secretSize, err := secrets.GetSecretSize(secret, deploymentDir)
			if err != nil {
				panic(err)
			}

			fmt.Fprintf(os.Stderr, "\t* %s (%d bytes).. ", secretName, secretSize)
			err = secrets.UploadSecret(ctx, host, secret, deploymentDir)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed")
				panic(err)
			} else {
				fmt.Fprintln(os.Stderr, "OK")
			}
		}
	}
}

func activateConfiguration(ctx ssh.Context, filteredHosts []nix.Host, resultPath string) {
	fmt.Fprintln(os.Stderr, "Executing '" + deploySwitchAction + "' on matched hosts:")
	fmt.Fprintln(os.Stderr)
	for _, host := range filteredHosts {

		fmt.Fprintln(os.Stderr, "** " + host.TargetHost)

		configuration, err := nix.GetNixSystemPath(host, resultPath)
		if err != nil {
			panic(err)
		}

		err = ctx.ActivateConfiguration(host, configuration, deploySwitchAction)
		if err != nil {
			panic(err)
		}

		fmt.Fprintln(os.Stderr)
	}
}
