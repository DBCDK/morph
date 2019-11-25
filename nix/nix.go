package nix

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dbcdk/morph/healthchecks"
	"github.com/dbcdk/morph/secrets"
	"github.com/dbcdk/morph/ssh"
	"github.com/dbcdk/morph/utils"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

type Host struct {
	HealthChecks            healthchecks.HealthChecks
	Name                    string
	NixosRelease            string
	TargetHost              string
	TargetUser              string
	Secrets                 map[string]secrets.Secret
	BuildOnly               bool
	SubstituteOnDestination bool
	NixConfig               map[string]string
}

type NixContext struct {
	EvalMachines string
	ShowTrace    bool
	KeepGCRoot   bool
}

func (host *Host) GetName() string {
	return host.Name
}

func (host *Host) GetTargetHost() string {
	return host.TargetHost
}

func (host *Host) GetTargetUser() string {
	return host.TargetUser
}

func (host *Host) GetHealthChecks() healthchecks.HealthChecks {
	return host.HealthChecks
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

func (ctx *NixContext) GetMachines(deploymentPath string) (hosts []Host, err error) {

	args := []string{"eval",
		"-f", ctx.EvalMachines, "info.machineList",
		"--arg", "networkExpr", deploymentPath,
		"--json"}

	if ctx.ShowTrace {
		args = append(args, "--show-trace")
	}

	cmd := exec.Command("nix", args...)

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
			"Error while running `nix eval ..`: %s", err.Error(),
		)
		return hosts, errors.New(errorMessage)
	}

	err = json.Unmarshal(stdout.Bytes(), &hosts)
	if err != nil {
		return hosts, err
	}

	return hosts, nil
}

func (ctx *NixContext) BuildMachines(deploymentPath string, hosts []Host, nixArgs []string, nixBuildTargets string) (resultPath string, err error) {
	hostsArg := "["
	for _, host := range hosts {
		hostsArg += "\"" + host.Name + "\" "
	}
	hostsArg += "]"

	resultLinkPath := filepath.Join(path.Dir(deploymentPath), ".gcroots", path.Base(deploymentPath))
	if ctx.KeepGCRoot {
		if err = os.MkdirAll(path.Dir(resultLinkPath), 0755); err != nil {
			ctx.KeepGCRoot = false
			fmt.Fprintf(os.Stderr, "Unable to create GC root, skipping: %s", err)
		}
	}
	if !ctx.KeepGCRoot {
		// create tmp dir for result link
		tmpdir, err := ioutil.TempDir("", "morph-")
		if err != nil {
			return "", err
		}
		utils.AddFinalizer(func() {
			os.RemoveAll(tmpdir)
		})
		resultLinkPath = filepath.Join(tmpdir, "result")
	}
	args := []string{ctx.EvalMachines,
		"-A", "machines",
		"--arg", "networkExpr", deploymentPath,
		"--arg", "names", hostsArg,
		"--out-link", resultLinkPath}

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

	cmd := exec.Command("nix-build", args...)

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
			"Error while running `nix build ...`: See above.",
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
	var userArg = ""
	var keyArg = ""
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
		env = append(env, fmt.Sprintf("NIX_SSHOPTS=%s", "-o StrictHostkeyChecking=No -o UserKnownHostsFile=/dev/null"))
	}

	options := mkOptions(host)
	for _, path := range paths {
		args := []string{
			"copy",
			path,
			"--to", "ssh://" + userArg + host.TargetHost + keyArg,
		}
		args = append(args, options...)
		if host.SubstituteOnDestination {
			args = append(args, "--substitute-on-destination")
		}

		cmd := exec.Command(
			"nix", args...,
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
