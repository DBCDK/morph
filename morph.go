package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DBCDK/kingpin"
	"github.com/DBCDK/morph/filter"
	"github.com/DBCDK/morph/healthchecks"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/planner"
	"github.com/DBCDK/morph/secrets"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
)

// These are set at build time via -ldflags magic
var version string
var assetRoot string

var switchActions = []string{"dry-activate", "test", "switch", "boot"}

var (
	app                 = kingpin.New("morph", "NixOS host manager").Version(version)
	dryRun              = app.Flag("dry-run", "Don't do anything, just eval and print changes").Default("False").Bool()
	selectGlob          string
	selectTags          string
	selectEvery         int
	selectSkip          int
	selectLimit         int
	orderingTags        string
	deployment          string
	timeout             int
	askForSudoPasswd    bool
	passCmd             string
	nixBuildArg         []string
	nixBuildTarget      string
	nixBuildTargetFile  string
	build               = buildCmd(app.Command("build", "Evaluate and build deployment configuration to the local Nix store"))
	eval                = evalCmd(app.Command("eval", "Inspect value of an attribute without building"))
	push                = pushCmd(app.Command("push", "Build and transfer items from the local Nix store to target machines"))
	deploy              = deployCmd(app.Command("deploy", "Build, push and activate new configuration on machines according to switch-action"))
	deploySwitchAction  string
	deployUploadSecrets bool
	deployReboot        bool
	skipHealthChecks    bool
	skipPreDeployChecks bool
	showTrace           bool
	healthCheck         = healthCheckCmd(app.Command("check-health", "Run health checks"))
	uploadSecrets       = uploadSecretsCmd(app.Command("upload-secrets", "Upload secrets"))
	listSecrets         = listSecretsCmd(app.Command("list-secrets", "List secrets"))
	asJson              bool
	attrkey             string
	execute             = executeCmd(app.Command("exec", "Execute arbitrary commands on machines"))
	executeCommand      []string
	keepGCRoot          = app.Flag("keep-result", "Keep latest build in .gcroots to prevent it from being garbage collected").Default("False").Bool()
	allowBuildShell     = app.Flag("allow-build-shell", "Allow using `network.buildShell` to build in a nix-shell which can execute arbitrary commands on the local system").Default("False").Bool()
	planOnly            = app.Flag("plan-only", "Print the execution plan and exit").Default("False").Bool()
	hostsMap            = make(map[string]nix.Host)
	cache               = make(map[string]string)
	cacheChan           = make(chan planner.StepData)
	stepsDone           = make(map[string]*sync.WaitGroup)
	stepsDoneChan       = make(chan planner.StepStatus)
)

func deploymentArg(cmd *kingpin.CmdClause) {
	cmd.Arg("deployment", "File containing the nix deployment expression").
		HintFiles("nix").
		Required().
		ExistingFileVar(&deployment)
}

func attributeArg(cmd *kingpin.CmdClause) {
	cmd.Arg("attribute", "Name of attribute to inspect").
		Required().
		StringVar(&attrkey)
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

func getSudoPasswdCommand(cmd *kingpin.CmdClause) {
	cmd.
		Flag("passcmd", "Specify command to run for sudo password").
		Default("").
		StringVar(&passCmd)
}

func selectorFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("on", "Glob for selecting servers in the deployment").
		Default("*").
		StringVar(&selectGlob)
	cmd.Flag("tagged", "Select hosts with these tags").
		Default("").
		StringVar(&selectTags)
	cmd.Flag("every", "Select every n hosts").
		Default("1").
		IntVar(&selectEvery)
	cmd.Flag("skip", "Skip first n hosts").
		Default("0").
		IntVar(&selectSkip)
	cmd.Flag("limit", "Select at most n hosts").
		IntVar(&selectLimit)
	cmd.Flag("order-by-tags", "Order hosts by tags (comma separated list)").
		Default("").
		StringVar(&orderingTags)
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

func skipPreDeployChecksFlag(cmd *kingpin.CmdClause) {
	cmd.
		Flag("skip-pre-deploy-checks", "Whether to skip all pre-deploy checks").
		Default("False").
		BoolVar(&skipPreDeployChecks)
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

func evalCmd(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	deploymentArg(cmd)
	attributeArg(cmd)
	return cmd
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
	getSudoPasswdCommand(cmd)
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
	getSudoPasswdCommand(cmd)
	skipHealthChecksFlag(cmd)
	skipPreDeployChecksFlag(cmd)
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
	getSudoPasswdCommand(cmd)
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
	utils.ValidateEnvironment("nix")

	utils.SignalHandler()

	if assetRoot == "" {
		handleError(errors.New("Morph must be compiled with \"-ldflags=-X main.assetRoot=<path-to-installed-data/>\"."))
	}
}

func main() {

	clause := kingpin.MustParse(app.Parse(os.Args[1:]))

	//TODO: Remove deprecation warning when removing --build-arg flag
	if len(nixBuildArg) > 0 {
		fmt.Fprintln(os.Stderr, "Deprecation: The --build-arg flag will be removed in a future release.")
	}

	defer utils.RunFinalizers()
	setup()

	// evaluate without building hosts
	switch clause {
	case eval.FullCommand():
		_, err := execEval()
		handleError(err)
		return
	}

	// setup hosts
	hosts, err := getHosts(deployment)
	handleError(err)

	go cacheWriter()
	go plannerStepStatusWriter()

	for _, host := range hosts {
		hostsMap[host.Name] = host
	}

	plan := planner.EmptyStep()
	plan.Description = "Root of execution plan"

	fmt.Println("Execution plan:")

	switch clause {
	case build.FullCommand():

		plan = planner.AddSteps(plan, planner.CreateBuildPlan(hosts))

	case push.FullCommand():

		build := planner.CreateBuildPlan(hosts)
		push := planner.CreatePushPlan(build.Id, hosts)
		plan = planner.AddSteps(plan, build, push)

	case deploy.FullCommand():
		fmt.Println("Execution plan: deploy: Not implemented")

	case healthCheck.FullCommand():
		fmt.Println("Execution plan: check-health: Not implemented")

	case uploadSecrets.FullCommand():
		fmt.Println("Execution plan: upload-secrets: Not implemented")

	case listSecrets.FullCommand():
		fmt.Println("Execution plan: list-secrets: Not implemented")

	case execute.FullCommand():
		fmt.Println("Execution plan: execute: Not implemented")

	}

	planJson, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(planJson))

	if *planOnly {
		// Don't execute the plan
		return
	}

	executePlan(plan)

	// switch clause {
	// case build.FullCommand():
	// 	_, err = execBuild(hosts)
	// case push.FullCommand():
	// 	_, err = execPush(hosts)
	// case deploy.FullCommand():
	// 	_, err = execDeploy(hosts)
	// case healthCheck.FullCommand():
	// 	err = execHealthCheck(hosts)
	// case uploadSecrets.FullCommand():
	// 	err = execUploadSecrets(createSSHContext(), hosts, nil)
	// case listSecrets.FullCommand():
	// 	if asJson {
	// 		err = execListSecretsAsJson(hosts)
	// 	} else {
	// 		execListSecrets(hosts)
	// 	}
	// case execute.FullCommand():
	// 	err = execExecute(hosts)
	// }

	// handleError(err)
}

func cacheWriter() {
	for update := range cacheChan {
		fmt.Printf("cache update: %s = %s\n", update.Key, update.Value)
		cache[update.Key] = update.Value
	}
}

func plannerStepStatusWriter() {
	for stepStatus := range stepsDoneChan {
		fmt.Printf("step update: %s = %s\n", stepStatus.Id, stepStatus.Status)
		switch strings.ToLower(stepStatus.Status) {
		case "started":
			var stepWg = &sync.WaitGroup{}
			stepWg.Add(1)

			stepsDone[stepStatus.Id] = stepWg
		case "done":
			stepsDone[stepStatus.Id].Done()

		default:
			panic("Only status=started and status=done allowed")
		}
	}
}

func executePlan(plan planner.Step) error {
	// not sure if anything special has to be done for the top-level step..

	return executeStep(plan)
}

func waitForDependencies(id string, hint string, dependencies []string) {
	fmt.Printf("%s: depends on %d steps: %v\n", id, len(dependencies), dependencies)

	for _, dependency := range dependencies {
		for {
			// Wait for the dependency to actually start running
			// It's probably better to pre-create all dependencies so this isn't necessary
			fmt.Printf("%s: %s: waiting for %s to start\n", id, hint, dependency)

			if dependencyWg, dependencyStarted := stepsDone[dependency]; dependencyStarted {
				// Wait for dependency to finish runningCheck if the dependencies update channel has been closed (=> it's done), and break the loop
				dependencyWg.Wait()
				fmt.Printf("%s: %s: %s done\n", id, hint, dependency)
				break

			}

			// Sleep if we haven't seen the dependency
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Println("hest")
}

func executeStep(step planner.Step) error {
	fmt.Printf("Running step %s: %s (dependencies: %v)\n", step.Action, step.Description, step.DependsOn)

	stepsDoneChan <- planner.StepStatus{Id: step.Id, Status: "started"}

	waitForDependencies(step.Id, "dependencies", step.DependsOn)

	switch step.Action {
	case "build":
		executeBuildStep(step)

	case "push":
		executePushStep(step)

	case "none":
		fallthrough
	case "":
		// wrapper step, nothing to do
	}

	// subStepIds := make([]string, 0)

	var wg sync.WaitGroup

	for _, subStep := range step.Steps {
		// subStepIds = append(subStepIds, subStep.Id)

		wg.Add(1)
		go func(step planner.Step) {
			defer wg.Done()
			executeStep(step)
		}(subStep)
	}

	// Wait for children to all be ready, before making the current step ready
	// waitForDependencies(step.Id, "childrenS", subStepIds)

	// FIXME: This will break when actually enabling multi-threading - this step has to wait for all sub steps to complete!

	wg.Wait()
	wg.Wait()
	fmt.Println("donkey")
	stepsDoneChan <- planner.StepStatus{Id: step.Id, Status: "done"}

	return nil
}

func executeBuildStep(step planner.Step) error {
	hosts := step.Options["hosts"].([]string)

	nixHosts := make([]nix.Host, 0)

	fmt.Println("Building hosts:")
	for _, host := range hosts {
		fmt.Printf("- %s\n", host)
		nixHosts = append(nixHosts, hostsMap[host])
	}

	resultPath, err := execBuild(nixHosts)
	fmt.Println(resultPath)

	for _, host := range hosts {
		hostPathSymlink := path.Join(resultPath, host)
		hostPath, err := filepath.EvalSymlinks(hostPathSymlink)
		if err != nil {
			return err
		}

		fmt.Println(hostPathSymlink)
		fmt.Println(hostPath)

		fmt.Println("ares1")
		cacheChan <- planner.StepData{Key: "closure:" + host, Value: hostPath}
		fmt.Println("ares2")

		// store hostPath to be fetched by other steps
	}

	return err
}

func executePushStep(step planner.Step) error {
	hostName := step.Options["to"].(string)
	host := hostsMap[hostName]
	cacheKey := "closure:" + host.Name
	fmt.Println("cache key: " + cacheKey)
	closure := cache[cacheKey]

	fmt.Printf("Pushing %s to %s\n", closure, host.TargetHost)

	sshContext := createSSHContext()
	err := nix.Push(sshContext, host, closure)

	return err
}

func handleError(err error) {
	//Stupid handling of catch-all errors for now
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
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

func execEval() (string, error) {
	ctx := getNixContext()

	deploymentFile, err := os.Open(deployment)
	deploymentPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return "", err
	}

	path, err := ctx.EvalHosts(deploymentPath, attrkey)

	return path, err
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
			phase := "pre-activation"
			err = execUploadSecrets(sshContext, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !skipPreDeployChecks {
			err := healthchecks.PerformPreDeployChecks(sshContext, &host, timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host pre-deploy check failed.")
				utils.Exit(1)
			}
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

		if doUploadSecrets {
			phase := "post-activation"
			err = execUploadSecrets(sshContext, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !skipHealthChecks {
			err := healthchecks.PerformHealthChecks(sshContext, &host, timeout)
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
		AskForSudoPassword:     askForSudoPasswd,
		GetSudoPasswordCommand: passCmd,
		IdentityFile:           os.Getenv("SSH_IDENTITY_FILE"),
		DefaultUsername:        os.Getenv("SSH_USER"),
		SkipHostKeyCheck:       os.Getenv("SSH_SKIP_HOST_KEY_CHECK") != "",
		ConfigFile:             os.Getenv("SSH_CONFIG_FILE"),
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
		err = healthchecks.PerformHealthChecks(sshContext, &host, timeout)
	}

	if err != nil {
		err = errors.New("One or more errors occurred during host healthchecks")
	}

	return err
}

func execUploadSecrets(sshContext *ssh.SSHContext, hosts []nix.Host, phase *string) error {
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Secret upload is disabled for build-only host: %s\n", host.Name)
			continue
		}
		singleHostInList := []nix.Host{host}

		err := secretsUpload(sshContext, singleHostInList, phase)
		if err != nil {
			return err
		}

		if !skipHealthChecks {
			err = healthchecks.PerformHealthChecks(sshContext, &host, timeout)
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

func getHosts(deploymentPath string) (hosts []nix.Host, err error) {

	deploymentFile, err := os.Open(deploymentPath)
	if err != nil {
		return hosts, err
	}

	deploymentAbsPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return hosts, err
	}

	ctx := getNixContext()
	deployment, err := ctx.GetMachines(deploymentAbsPath)
	if err != nil {
		return hosts, err
	}

	matchingHosts, err := filter.MatchHosts(deployment.Hosts, selectGlob)
	if err != nil {
		return hosts, err
	}

	var selectedTags []string
	if selectTags != "" {
		selectedTags = strings.Split(selectTags, ",")
	}

	matchingHosts2 := filter.FilterHostsTags(matchingHosts, selectedTags)

	ordering := deployment.Meta.Ordering
	if orderingTags != "" {
		ordering = nix.HostOrdering{Tags: strings.Split(orderingTags, ",")}
	}

	sortedHosts := filter.SortHosts(matchingHosts2, ordering)

	filteredHosts := filter.FilterHosts(sortedHosts, selectSkip, selectEvery, selectLimit)

	fmt.Fprintf(os.Stderr, "Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(deployment.Hosts), len(deployment.Hosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "\t%3d: %s (secrets: %d, health checks: %d, tags: %s)\n", index, host.Name, len(host.Secrets), len(host.HealthChecks.Cmd)+len(host.HealthChecks.Http), strings.Join(host.GetTags(), ","))
	}
	fmt.Fprintln(os.Stderr)

	return filteredHosts, nil
}

func getNixContext() *nix.NixContext {
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
		evalMachines = filepath.Join(assetRoot, "eval-machines.nix")
	}

	return &nix.NixContext{
		EvalCmd:         evalCmd,
		BuildCmd:        buildCmd,
		ShellCmd:        shellCmd,
		EvalMachines:    evalMachines,
		ShowTrace:       showTrace,
		KeepGCRoot:      *keepGCRoot,
		AllowBuildShell: *allowBuildShell,
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

func secretsUpload(ctx ssh.Context, filteredHosts []nix.Host, phase *string) error {
	// upload secrets
	// relative paths are resolved relative to the deployment file (!)
	deploymentDir := filepath.Dir(deployment)
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
