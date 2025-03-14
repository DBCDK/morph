package cliparser

import (
	"strings"

	"github.com/DBCDK/kingpin"
	"github.com/DBCDK/morph/common"
)

type KingpinCmdClauses struct {
	Build         *kingpin.CmdClause
	Deploy        *kingpin.CmdClause
	Eval          *kingpin.CmdClause
	Execute       *kingpin.CmdClause
	HealthCheck   *kingpin.CmdClause
	Push          *kingpin.CmdClause
	SecretsUpload *kingpin.CmdClause
	SecretsList   *kingpin.CmdClause
}

func New(version string, assetRoot string) (*kingpin.Application, *KingpinCmdClauses, *common.MorphOptions) {
	app := kingpin.New("morph", "NixOS host manager").Version(version)

	options := &common.MorphOptions{
		Version:   version,
		AssetRoot: assetRoot,

		DryRun:          app.Flag("dry-run", "Don't do anything, just eval and print changes").Default("False").Bool(),
		JsonOut:         app.Flag("i-know-kung-fu", "Output as JSON").Default("False").Bool(),
		ConstraintsFlag: app.Flag("constraint", "Add constraints to manipulate order of execution").Default("").Strings(),
		KeepGCRoot:      app.Flag("keep-result", "Keep latest build in .gcroots to prevent it from being garbage collected").Default("False").Bool(),
		AllowBuildShell: app.Flag("allow-build-shell", "Allow using `network.buildShell` to build in a nix-shell which can execute arbitrary commands on the local system").Default("False").Bool(),
	}

	cmdClauses := &KingpinCmdClauses{
		Build:         buildCmd(app.Command("build", "Evaluate and build deployment configuration to the local Nix store"), options),
		Deploy:        deployCmd(app.Command("deploy", "Build, push and activate new configuration on machines according to switch-action"), options),
		Eval:          evalCmd(app.Command("eval", "Inspect value of an attribute without building"), options),
		Execute:       executeCmd(app.Command("exec", "Execute arbitrary commands on machines"), options),
		HealthCheck:   healthCheckCmd(app.Command("check-health", "Run health checks"), options),
		Push:          pushCmd(app.Command("push", "Build and transfer items from the local Nix store to target machines"), options),
		SecretsList:   listSecretsCmd(app.Command("list-secrets", "List secrets"), options),
		SecretsUpload: uploadSecretsCmd(app.Command("upload-secrets", "Upload secrets"), options),
	}

	return app, cmdClauses, options
}

func deploymentArg(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.Arg("deployment", "File containing the nix deployment expression").
		HintFiles("nix").
		Required().
		ExistingFileVar(&cfg.Deployment)
}

func attributeArg(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.Arg("attribute", "Name of attribute to inspect").
		Required().
		StringVar(&cfg.AttrKey)
}

func timeoutFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.Flag("timeout", "Seconds to wait for commands/healthchecks on a host to complete").
		Default("0").
		IntVar(&cfg.Timeout)
}

func askForSudoPasswdFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.
		Flag("passwd", "Whether to ask interactively for remote sudo password when needed").
		Default("False").
		BoolVar(&cfg.AskForSudoPasswd)
}

func getSudoPasswdCommand(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.
		Flag("passcmd", "Specify command to run for sudo password").
		Default("").
		StringVar(&cfg.PassCmd)
}

func selectorFlags(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.Flag("on", "Glob for selecting servers in the deployment").
		Default("*").
		StringVar(&cfg.SelectGlob)
	cmd.Flag("tagged", "Select hosts with these tags").
		Default("").
		StringVar(&cfg.SelectTags)
	cmd.Flag("every", "Select every n hosts").
		Default("1").
		IntVar(&cfg.SelectEvery)
	cmd.Flag("skip", "Skip first n hosts").
		Default("0").
		IntVar(&cfg.SelectSkip)
	cmd.Flag("limit", "Select at most n hosts").
		IntVar(&cfg.SelectLimit)
	cmd.Flag("order-by-tags", "Order hosts by tags (comma separated list)").
		Default("").
		StringVar(&cfg.OrderingTags)
}

func nixBuildTargetFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.Flag("target", "A Nix lambda defining the build target to use instead of the default").
		StringVar(&cfg.NixBuildTarget)
}

func nixBuildTargetFileFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.Flag("target-file", "File containing a Nix attribute set, defining build targets to use instead of the default").
		HintFiles("nix").
		ExistingFileVar(&cfg.NixBuildTargetFile)
}

func skipHealthChecksFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.
		Flag("skip-health-checks", "Whether to skip all health checks").
		Default("False").
		BoolVar(&cfg.SkipHealthChecks)
}

func skipPreDeployChecksFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.
		Flag("skip-pre-deploy-checks", "Whether to skip all pre-deploy checks").
		Default("False").
		BoolVar(&cfg.SkipPreDeployChecks)
}

func showTraceFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.
		Flag("show-trace", "Whether to pass --show-trace to all nix commands").
		Default("False").
		BoolVar(&cfg.ShowTrace)
}

func asJsonFlag(cmd *kingpin.CmdClause, cfg *common.MorphOptions) {
	cmd.
		Flag("json", "Whether to format the output as JSON instead of plaintext").
		Default("False").
		BoolVar(&cfg.AsJson)
}

func evalCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	deploymentArg(cmd, cfg)
	attributeArg(cmd, cfg)
	return cmd
}

func buildCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	nixBuildTargetFlag(cmd, cfg)
	nixBuildTargetFileFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	return cmd
}

func pushCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	return cmd
}

func executeCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	askForSudoPasswdFlag(cmd, cfg)
	getSudoPasswdCommand(cmd, cfg)
	timeoutFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	cmd.
		Arg("command", "Command to execute").
		Required().
		StringsVar(&cfg.ExecuteCommand)
	cmd.NoInterspersed = true
	return cmd
}

func deployCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	switchActions := []string{"dry-activate", "test", "switch", "boot"}

	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	timeoutFlag(cmd, cfg)
	askForSudoPasswdFlag(cmd, cfg)
	getSudoPasswdCommand(cmd, cfg)
	skipHealthChecksFlag(cmd, cfg)
	skipPreDeployChecksFlag(cmd, cfg)
	cmd.
		Flag("upload-secrets", "Upload secrets as part of the host deployment").
		Default("False").
		BoolVar(&cfg.DeployUploadSecrets)
	cmd.
		Flag("reboot", "Reboots the host after system activation, but before healthchecks has executed.").
		Default("False").
		BoolVar(&cfg.DeployReboot)
	cmd.
		Arg("switch-action", "Either of "+strings.Join(switchActions, "|")).
		Required().
		HintOptions(switchActions...).
		EnumVar(&cfg.DeploySwitchAction, switchActions...)
	return cmd
}

func healthCheckCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	timeoutFlag(cmd, cfg)
	return cmd
}

func uploadSecretsCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	askForSudoPasswdFlag(cmd, cfg)
	getSudoPasswdCommand(cmd, cfg)
	skipHealthChecksFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	return cmd
}

func listSecretsCmd(cmd *kingpin.CmdClause, cfg *common.MorphOptions) *kingpin.CmdClause {
	selectorFlags(cmd, cfg)
	showTraceFlag(cmd, cfg)
	deploymentArg(cmd, cfg)
	asJsonFlag(cmd, cfg)
	return cmd
}
