package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dbcdk/kingpin"
	"github.com/dbcdk/morph/assets"
	"github.com/dbcdk/morph/filter"
	"github.com/dbcdk/morph/healthchecks"
	"github.com/dbcdk/morph/nix"
	"github.com/dbcdk/morph/secrets"
	"github.com/dbcdk/morph/ssh"
	"github.com/dbcdk/morph/utils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// This is set at build time via -ldflags magic
var version string
var switchActions = []string{"dry-activate", "test", "switch", "boot"}

var (
	app                 = kingpin.New("morph", "NixOS host manager").Version(version)
	dryRun              = app.Flag("dry-run", "Don't do anything, just eval and print changes").Default("False").Bool()
	selectGlob          string
	selectEvery         int
	selectSkip          int
	selectLimit         int
	deployment          string
	timeout             int
	askForSudoPasswd    bool
	nixBuildArg         []string
	nixBuildTarget      string
	nixBuildTargetFile  string
	build               = buildCmd(app.Command("build", "Evaluate and build deployment configuration to the local Nix store"))
	push                = pushCmd(app.Command("push", "Build and transfer items from the local Nix store to target machines"))
	deploy              = deployCmd(app.Command("deploy", "Build, push and activate new configuration on machines according to switch-action"))
	deploySwitchAction  string
	deployUploadSecrets bool
	deployReboot        bool
	skipHealthChecks    bool
	showTrace           bool
	healthCheck         = healthCheckCmd(app.Command("check-health", "Run health checks"))
	uploadSecrets       = uploadSecretsCmd(app.Command("upload-secrets", "Upload secrets"))
	listSecrets         = listSecretsCmd(app.Command("list-secrets", "List secrets"))
	asJson              bool
	execute             = executeCmd(app.Command("exec", "Execute arbitrary commands on machines"))
	executeCommand      []string
	keepGCRoot          = app.Flag("keep-result", "Keep latest build in .gcroots to prevent it from being garbage collected").Default("False").Bool()

	assetRoot string
)

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

func nixBuildArgFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("build-arg", "Extra argument to pass on to nix-build command. **DEPRECATED**").
		StringsVar(&nixBuildArg)
}

func nixBuildTargetFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("target", "A Nix lambda defining the build target to use instead of the default").
		StringVar(&nixBuildTarget)
}

func nixBuildTargetFileFlag(cmd *kingpin.CmdClause) {
	cmd.Flag("target-file", "File containing a Nix attribute set, defining build targets to use instead of the default").
		HintFiles("nix").
		ExistingFileVar(&nixBuildTargetFile)
}

func skipHealthChecksFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("skip-health-checks", "Whether to skip all health checks").
		Default("False").
		BoolVar(&skipHealthChecks)
}

func showTraceFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("show-trace", "Whether to pass --show-trace to all nix commands").
		Default("False").
		BoolVar(&showTrace)
}

func asJsonFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("json", "Whether to format the output as JSON instead of plaintext").
		Default("False").
		BoolVar(&asJson)
}

func buildCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	nixBuildArgFlag(cmd)
	nixBuildTargetFlag(cmd)
	nixBuildTargetFileFlag(cmd)
	deploymentArg(cmd)
	return cmd
}

func pushCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	deploymentArg(cmd)
	return cmd
}

func executeCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
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
	showTraceFlag(cmd)
	nixBuildArgFlag(cmd)
	deploymentArg(cmd)
	timeoutFlag(cmd)
	askForSudoPasswdFlag(cmd)
	skipHealthChecksFlag(cmd)
	cmd.
		Flag("upload-secrets", "Upload secrets as part of the host deployment").
		Default("False").
		BoolVar(&deployUploadSecrets)
	cmd.
		Flag("reboot", "Reboots the host after system activation, but before healthchecks has executed.").
		Default("False").
		BoolVar(&deployReboot)
	cmd.
		Arg("switch-action", "Either of "+strings.Join(switchActions, "|")).
		Required().
		HintOptions(switchActions...).
		EnumVar(&deploySwitchAction, switchActions...)
	return cmd
}

func healthCheckCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	deploymentArg(cmd)
	timeoutFlag(cmd)
	return cmd
}

func uploadSecretsCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	askForSudoPasswdFlag(cmd)
	skipHealthChecksFlag(cmd)
	deploymentArg(cmd)
	return cmd
}

func listSecretsCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	selectorFlags(cmd)
	showTraceFlag(cmd)
	deploymentArg(cmd)
	asJsonFlag(cmd)
	return cmd
}

func setup() {
	handleError(validateEnvironment())

	utils.AddFinalizer(func() {
		assets.Teardown(assetRoot)
	})
	utils.SignalHandler()

	var assetErr error
	assetRoot, assetErr = assets.Setup()
	handleError(assetErr)
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

func main() {

	clause := kingpin.MustParse(app.Parse(os.Args[1:]))

	//TODO: Remove deprecation warning when removing --build-arg flag
	if len(nixBuildArg) > 0 {
		fmt.Fprintln(os.Stderr, "Deprecation: The --build-arg flag will be removed in a future release.")
	}

	defer utils.RunFinalizers()
	setup()

	hosts, err := getHosts(deployment)
	handleError(err)

	switch clause {
	case build.FullCommand():
		_, err = execBuild(hosts)
	case push.FullCommand():
		_, err = execPush(hosts)
	case deploy.FullCommand():
		_, err = execDeploy(hosts)
	case healthCheck.FullCommand():
		err = execHealthCheck(hosts)
	case uploadSecrets.FullCommand():
		err = execUploadSecrets(createSSHContext(), hosts)
	case listSecrets.FullCommand():
		if asJson {
			err = execListSecretsAsJson(hosts)
		} else {
			execListSecrets(hosts)
		}
	case execute.FullCommand():
		err = execExecute(hosts)
	}

	handleError(err)
}

func handleError(err error) {
	//Stupid handling of catch-all errors for now
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		utils.Exit(1)
	}
}

func execExecute(hosts []nix.Host) error {
	sshContext := createSSHContext()

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Exec is disabled for build-only host: %s\n", host.Name)
			continue
		}
		fmt.Fprintln(os.Stderr, "** "+host.Name)
		sshContext.CmdInteractive(&host, timeout, executeCommand...)
		fmt.Fprintln(os.Stderr)
	}

	return nil
}

func execBuild(hosts []nix.Host) (string, error) {
	resultPath, err := buildHosts(hosts)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

func execPush(hosts []nix.Host) (string, error) {
	resultPath, err := execBuild(hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)
	return resultPath, pushPaths(createSSHContext(), hosts, resultPath)
}

func execDeploy(hosts []nix.Host) (string, error) {
	doPush := false
	doUploadSecrets := false
	doActivate := false

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
			doUploadSecrets = deployUploadSecrets
			doActivate = true
		}
	}

	resultPath, err := buildHosts(hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)

	sshContext := createSSHContext()

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
			err = execUploadSecrets(sshContext, singleHostInList)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if doActivate {
			err = activateConfiguration(sshContext, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}

		if deployReboot {
			err = host.Reboot(sshContext)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Reboot failed")
				return "", err
			}
		}

		if !skipHealthChecks {
			err := healthchecks.Perform(sshContext, &host, timeout)
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

func createSSHContext() *ssh.SSHContext {
	return &ssh.SSHContext{
		AskForSudoPassword: askForSudoPasswd,
		IdentityFile:       os.Getenv("SSH_IDENTITY_FILE"),
		DefaultUsername:    os.Getenv("SSH_USER"),
		SkipHostKeyCheck:   os.Getenv("SSH_SKIP_HOST_KEY_CHECK") != "",
	}
}

func execHealthCheck(hosts []nix.Host) error {
	sshContext := createSSHContext()

	var err error
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Healthchecks are disabled for build-only host: %s\n", host.Name)
			continue
		}
		err = healthchecks.Perform(sshContext, &host, timeout)
	}

	if err != nil {
		err = errors.New("One or more errors occurred during host healthchecks")
	}

	return err
}

func execUploadSecrets(sshContext *ssh.SSHContext, hosts []nix.Host) error {
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Secret upload is disabled for build-only host: %s\n", host.Name)
			continue
		}
		singleHostInList := []nix.Host{host}

		err := secretsUpload(sshContext, singleHostInList)
		if err != nil {
			return err
		}

		if !skipHealthChecks {
			err = healthchecks.Perform(sshContext, &host, timeout)
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

func execListSecretsAsJson(hosts []nix.Host) error {
	deploymentDir, err := filepath.Abs(filepath.Dir(deployment))
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

func getHosts(deploymentFile string) (hosts []nix.Host, err error) {

	deployment, err := os.Open(deploymentFile)
	if err != nil {
		return hosts, err
	}

	deploymentPath, err := filepath.Abs(deployment.Name())
	if err != nil {
		return hosts, err
	}

	ctx := getNixContext()
	allHosts, err := ctx.GetMachines(deploymentPath)
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
		fmt.Fprintf(os.Stderr, "\t%3d: %s (secrets: %d, health checks: %d)\n", index, host.Name, len(host.Secrets), len(host.HealthChecks.Cmd)+len(host.HealthChecks.Http))
	}
	fmt.Fprintln(os.Stderr)

	return filteredHosts, nil
}

func getNixContext() *nix.NixContext {
	return &nix.NixContext{
		EvalMachines: filepath.Join(assetRoot, "eval-machines.nix"),
		ShowTrace:    showTrace,
		KeepGCRoot:   *keepGCRoot,
	}
}

func buildHosts(hosts []nix.Host) (resultPath string, err error) {
	if len(hosts) == 0 {
		err = errors.New("No hosts selected")
		return
	}

	deploymentPath, err := filepath.Abs(deployment)
	if err != nil {
		return
	}

	nixBuildTargets := ""
	if nixBuildTargetFile != "" {
		if path, err := filepath.Abs(nixBuildTargetFile); err == nil {
			nixBuildTargets = fmt.Sprintf("import \"%s\"", path)
		}
	} else if nixBuildTarget != "" {
		nixBuildTargets = fmt.Sprintf("{ \"out\" = %s; }", nixBuildTarget)
	}

	ctx := getNixContext()
	resultPath, err = ctx.BuildMachines(deploymentPath, hosts, nixBuildArg, nixBuildTargets)

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

func secretsUpload(ctx ssh.Context, filteredHosts []nix.Host) error {
	// upload secrets
	// relative paths are resolved relative to the deployment file (!)
	deploymentDir := filepath.Dir(deployment)
	for _, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "Uploading secrets to %s (%s):\n", host.Name, host.TargetHost)
		postUploadActions := make(map[string][]string, 0)
		for secretName, secret := range host.Secrets {
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
			ctx.CmdInteractive(&host, timeout, action...)
		}
	}

	return nil
}

func activateConfiguration(ctx ssh.Context, filteredHosts []nix.Host, resultPath string) error {
	fmt.Fprintln(os.Stderr, "Executing '"+deploySwitchAction+"' on matched hosts:")
	fmt.Fprintln(os.Stderr)
	for _, host := range filteredHosts {

		fmt.Fprintln(os.Stderr, "** "+host.Name)

		configuration, err := nix.GetNixSystemPath(host, resultPath)
		if err != nil {
			return err
		}

		err = ctx.ActivateConfiguration(&host, configuration, deploySwitchAction)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr)
	}

	return nil
}
