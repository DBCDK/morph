package nix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DBCDK/morph/healthchecks"
	"github.com/DBCDK/morph/secrets"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
)

type Host struct {
	HealthChecks            healthchecks.HealthChecks
	Name                    string
	NixosRelease            string
	TargetHost              string
	TargetPort              int
	TargetUser              string
	Secrets                 map[string]secrets.Secret
	BuildOnly               bool
	SubstituteOnDestination bool
	NixConfig               map[string]string
	Tags                    []string
}

type HostOrdering struct {
	Tags []string
}

type DeploymentMetadata struct {
	Description string
	Ordering    HostOrdering
}

type Deployment struct {
	Hosts []Host             `json:"hosts"`
	Meta  DeploymentMetadata `json:"meta"`
}

type NixContext struct {
	EvalMachines    string
	ShowTrace       bool
	KeepGCRoot      bool
	AllowBuildShell bool
}

type FileArgs struct {
	Names []string
}

func (host *Host) GetName() string {
	return host.Name
}

func (host *Host) GetTargetHost() string {
	return host.TargetHost
}

func (host *Host) GetTargetPort() int {
	return host.TargetPort
}

func (host *Host) GetTargetUser() string {
	return host.TargetUser
}

func (host *Host) GetHealthChecks() healthchecks.HealthChecks {
	return host.HealthChecks
}

func (host *Host) GetTags() []string {
	return host.Tags
}

func (host *Host) Reboot(sshContext *ssh.SSHContext) error {

	var (
		oldBootID string
		newBootID string
	)

	oldBootID, err := sshContext.GetBootID(host)
	// If the host doesn't support getting boot ID's for some reason, warn about it, and skip the comparison
	skipBootIDComparison := err != nil
	if skipBootIDComparison {
		fmt.Fprintf(os.Stderr, "Error getting boot ID (this is used to determine when the reboot is complete): %v\n", err)
		fmt.Fprintf(os.Stderr, "This makes it impossible to detect when the host has rebooted, so health checks might pass before the host has rebooted.\n")
	}

	if cmd, err := sshContext.Cmd(host, "sudo", "reboot"); cmd != nil {
		fmt.Fprint(os.Stderr, "Asking host to reboot ... ")
		if err = cmd.Run(); err != nil {
			// Here we assume that exit code 255 means: "SSH connection got disconnected",
			// which is OK for a reboot - sshd may close active connections before we disconnect after all
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() == 255 {
					fmt.Fprintln(os.Stderr, "Remote host disconnected.")
					err = nil
				}
			}
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed")
			return err
		}
	}

	fmt.Fprintln(os.Stderr, "OK")

	if !skipBootIDComparison {
		fmt.Fprint(os.Stderr, "Waiting for host to come online ")

		// Wait for the host to get a new boot ID. These ID's should be unique for each boot,
		// meaning a reboot will have been completed when the boot ID has changed.
		for {
			fmt.Fprint(os.Stderr, ".")

			// Ignore errors; there'll be plenty of them since we'll be attempting to connect to an offline host,
			// and we know from previously that the host should support boot ID's
			newBootID, _ = sshContext.GetBootID(host)

			if newBootID != "" && oldBootID != newBootID {
				fmt.Fprintln(os.Stderr, " OK")
				break
			}

			time.Sleep(2 * time.Second)
		}
	}

	return nil
}

func (ctx *NixContext) GetBuildShell(deploymentPath string) (buildShell *string, err error) {

	args := []string{"--eval", ctx.EvalMachines,
		"--attr", "info.buildShell",
		"--arg", "networkExpr", deploymentPath,
		"--json", "--strict", "--read-write-mode"}

	if ctx.ShowTrace {
		args = append(args, "--show-trace")
	}

	cmd := exec.Command("nix-instantiate", args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	utils.AddFinalizer(func() {
		if (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	})
	err = cmd.Run()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error while running `nix-instantiate ..`: %s", err.Error(),
		)
		return buildShell, errors.New(errorMessage)
	}

	err = json.Unmarshal(stdout.Bytes(), &buildShell)
	if err != nil {
		return nil, err
	}

	return buildShell, nil
}

func (ctx *NixContext) EvalHosts(deploymentPath string, attr string) (string, error) {
	attribute := "nodes." + attr
	args := []string{ctx.EvalMachines,
		"--arg", "networkExpr", deploymentPath,
		"--eval", "--strict", "-A", attribute}

	cmd := exec.Command("nix-instantiate", args...)
	utils.AddFinalizer(func() {
		if (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	})

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return deploymentPath, err
}

func (ctx *NixContext) GetMachines(deploymentPath string) (deployment Deployment, err error) {

	args := []string{"--eval", ctx.EvalMachines,
		"--attr", "info.deployment",
		"--arg", "networkExpr", deploymentPath,
		"--json", "--strict"}

	if ctx.ShowTrace {
		args = append(args, "--show-trace")
	}

	cmd := exec.Command("nix-instantiate", args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	utils.AddFinalizer(func() {
		if (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	})
	err = cmd.Run()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error while running `nix-instantiate ..`: %s", err.Error(),
		)
		return deployment, errors.New(errorMessage)
	}

	err = json.Unmarshal(stdout.Bytes(), &deployment)
	if err != nil {
		return deployment, err
	}

	return deployment, nil
}

func (ctx *NixContext) BuildMachines(deploymentPath string, hosts []Host, nixArgs []string, nixBuildTargets string) (resultPath string, err error) {
	tmpdir, err := ioutil.TempDir("", "morph-")
	if err != nil {
		return "", err
	}
	utils.AddFinalizer(func() {
		os.RemoveAll(tmpdir)
	})

	hostsArg := []string{}
	for _, host := range hosts {
		hostsArg = append(hostsArg, host.Name)
	}

	fileArgs := FileArgs{
		Names: hostsArg,
	}

	jsonArgs, err := json.Marshal(fileArgs)
	if err != nil {
		return "", err
	}
	argsFile := tmpdir + "/morph-args.json"

	err = ioutil.WriteFile(argsFile, jsonArgs, 0644)
	if err != nil {
		return "", err
	}

	resultLinkPath := filepath.Join(path.Dir(deploymentPath), ".gcroots", path.Base(deploymentPath))
	if ctx.KeepGCRoot {
		if err = os.MkdirAll(path.Dir(resultLinkPath), 0755); err != nil {
			ctx.KeepGCRoot = false
			fmt.Fprintf(os.Stderr, "Unable to create GC root, skipping: %s", err)
		}
	}
	if !ctx.KeepGCRoot {
		// create tmp dir for result link
		resultLinkPath = filepath.Join(tmpdir, "result")
	}
	args := []string{
		ctx.EvalMachines,
		"--arg", "networkExpr", deploymentPath,
		"--argstr", "argsFile", argsFile,
		"--out-link", resultLinkPath,
		"--attr", "machines",
	}

	args = append(args, mkOptions(hosts[0])...)

	if len(nixArgs) > 0 {
		args = append(args, nixArgs...)
	}

	if ctx.ShowTrace {
		args = append(args, "--show-trace")
	}

	if nixBuildTargets != "" {
		args = append(args,
			"--arg", "buildTargets", nixBuildTargets)
	}

	buildShell, err := ctx.GetBuildShell(deploymentPath)

	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error getting buildShell.",
		)
		return resultPath, errors.New(errorMessage)
	}

	var cmd *exec.Cmd
	if ctx.AllowBuildShell && buildShell != nil {
		shellArgs := strings.Join(append([]string{"nix-build"}, args...), " ")
		cmd = exec.Command("nix-shell", *buildShell, "--pure", "--run", shellArgs)
	} else {
		cmd = exec.Command("nix-build", args...)
	}

	// show process output on attached stdout/stderr
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	utils.AddFinalizer(func() {
		if (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	})
	err = cmd.Run()

	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error while running `%s ...`: See above.", cmd.String(),
		)
		return resultPath, errors.New(errorMessage)
	}

	resultPath, err = os.Readlink(resultLinkPath)
	if err != nil {
		return "", err
	}

	return
}

func mkOptions(host Host) []string {
	var options = make([]string, 0)
	for k, v := range host.NixConfig {
		options = append(options, "--option")
		options = append(options, k)
		options = append(options, v)
	}
	return options
}

func GetNixSystemPath(host Host, resultPath string) (string, error) {
	return os.Readlink(filepath.Join(resultPath, host.Name))
}

func GetNixSystemDerivation(host Host, resultPath string) (string, error) {
	return os.Readlink(filepath.Join(resultPath, host.Name+".drv"))
}

func GetPathsToPush(host Host, resultPath string) (paths []string, err error) {
	path1, err := GetNixSystemPath(host, resultPath)
	if err != nil {
		return paths, err
	}

	paths = append(paths, path1)

	return paths, nil
}

func Push(ctx *ssh.SSHContext, host Host, paths ...string) (err error) {
	utils.ValidateEnvironment("ssh")

	var userArg = ""
	var keyArg = ""
	var sshOpts = []string{}
	var env = os.Environ()
	if host.TargetUser != "" {
		userArg = host.TargetUser + "@"
	} else if ctx.DefaultUsername != "" {
		userArg = ctx.DefaultUsername + "@"
	}
	if ctx.IdentityFile != "" {
		keyArg = "?ssh-key=" + ctx.IdentityFile
	}
	if ctx.SkipHostKeyCheck {
		sshOpts = append(sshOpts, fmt.Sprintf("%s", "-o StrictHostkeyChecking=No -o UserKnownHostsFile=/dev/null"))
	}
	if host.TargetPort != 0 {
		sshOpts = append(sshOpts, fmt.Sprintf("-p %d", host.TargetPort))
	}
	if ctx.ConfigFile != "" {
		sshOpts = append(sshOpts, fmt.Sprintf("-F %s", ctx.ConfigFile))
	}
	if len(sshOpts) > 0 {
		env = append(env, fmt.Sprintf("NIX_SSHOPTS=%s", strings.Join(sshOpts, " ")))
	}

	options := mkOptions(host)
	for _, path := range paths {
		args := []string{
			"--to", userArg + host.TargetHost + keyArg,
			path,
		}
		args = append(args, options...)
		if host.SubstituteOnDestination {
			args = append(args, "--use-substitutes")
		}

		cmd := exec.Command(
			"nix-copy-closure", args...,
		)
		cmd.Env = env

		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		err = cmd.Run()

		if err != nil {
			return err
		}
	}

	return nil
}
