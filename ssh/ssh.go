package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/DBCDK/morph/utils"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Context interface {
	ActivateConfiguration(host Host, configuration string, action string) error
	MakeTempFile(host Host) (path string, err error)
	UploadFile(host Host, source string, destination string) error
	SetOwner(host Host, path string, user string, group string) error
	SetPermissions(host Host, path string, permissions string) error
	MoveFile(host Host, source string, destination string) error
	MakeDirs(host Host, path string, parents bool, mode os.FileMode) error

	Cmd(host Host, parts ...string) (*exec.Cmd, error)
	SudoCmd(host Host, parts ...string) (*exec.Cmd, error)
	CmdInteractive(host Host, timeout int, parts ...string)
}

type Host interface {
	GetName() string
	GetTargetHost() string
	GetTargetUser() string
}

type SSHContext struct {
	sudoPassword       string
	AskForSudoPassword bool
	DefaultUsername    string
	IdentityFile       string
	ConfigFile         string
	SkipHostKeyCheck   bool
}

type FileTransfer struct {
	Source      string
	Destination string
}

func (sshCtx *SSHContext) Cmd(host Host, parts ...string) (*exec.Cmd, error) {
	return sshCtx.CmdContext(context.TODO(), host, parts...)
}

func (sshCtx *SSHContext) CmdContext(ctx context.Context, host Host, parts ...string) (*exec.Cmd, error) {

	var err error
	if parts, err = valCommand(parts); err != nil {
		return nil, err
	}

	if parts[0] == "sudo" {
		return sshCtx.SudoCmdContext(ctx, host, parts...)
	}

	cmd, cmdArgs := sshCtx.sshArgs(host, nil)
	cmdArgs = append(cmdArgs, parts...)

	command := exec.CommandContext(ctx, cmd, cmdArgs...)
	return command, nil
}

func (ctx *SSHContext) sshArgs(host Host, transfer *FileTransfer) (cmd string, args []string) {
	if transfer != nil {
		cmd = "scp"
	} else {
		cmd = "ssh"
	}
	utils.ValidateEnvironment(cmd)

	if ctx.SkipHostKeyCheck {
		args = append(args,
			"-o", "StrictHostKeyChecking=No",
			"-o", "UserKnownHostsFile=/dev/null")
	}
	if ctx.IdentityFile != "" {
		args = append(args, "-i")
		args = append(args, ctx.IdentityFile)
	}
	if ctx.ConfigFile != "" {
		args = append(args, "-F", ctx.ConfigFile)
	}
	var hostAndDestination = host.GetTargetHost()
	if transfer != nil {
		args = append(args, transfer.Source)
		hostAndDestination += ":" + transfer.Destination
	}
	if host.GetTargetUser() != "" {
		hostAndDestination = host.GetTargetUser() + "@" + hostAndDestination
	} else if ctx.DefaultUsername != "" {
		hostAndDestination = ctx.DefaultUsername + "@" + hostAndDestination
	}
	args = append(args, hostAndDestination)

	return
}

func (sshCtx *SSHContext) SudoCmd(host Host, parts ...string) (*exec.Cmd, error) {
	return sshCtx.SudoCmdContext(context.TODO(), host, parts...)
}

func (sshCtx *SSHContext) SudoCmdContext(ctx context.Context, host Host, parts ...string) (*exec.Cmd, error) {
	var err error
	if parts, err = valCommand(parts); err != nil {
		return nil, err
	}

	// ask for password if not done already
	if sshCtx.AskForSudoPassword && sshCtx.sudoPassword == "" {
		sshCtx.sudoPassword, err = askForSudoPassword()
		if err != nil {
			return nil, err
		}
	}

	cmd, cmdArgs := sshCtx.sshArgs(host, nil)

	// normalize sudo
	if parts[0] == "sudo" {
		parts = parts[1:]
	}
	cmdArgs = append(cmdArgs, "sudo")

	if sshCtx.sudoPassword != "" {
		cmdArgs = append(cmdArgs, "-S")
	} else {
		// no password supplied; request non-interactive sudo, which will fail with an error if a password was required
		cmdArgs = append(cmdArgs, "-n")
	}

	cmdArgs = append(cmdArgs, "-p", "''", "-k", "--")
	cmdArgs = append(cmdArgs, parts...)

	command := exec.CommandContext(ctx, cmd, cmdArgs...)
	if sshCtx.sudoPassword != "" {
		err := writeSudoPassword(command, sshCtx.sudoPassword)
		if err != nil {
			return nil, err
		}
	}
	return command, nil
}

func valCommand(parts []string) ([]string, error) {

	if len(parts) < 1 {
		return nil, errors.New("No command specified")
	}

	return parts, nil
}

func (sshCtx *SSHContext) CmdInteractive(host Host, timeout int, parts ...string) {
	ctx, cancel := utils.ContextWithConditionalTimeout(context.TODO(), timeout)
	defer cancel()

	cmd, err := sshCtx.CmdContext(ctx, host, parts...)
	if err == nil {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	}

	// context was cancelled
	if ctx.Err() != nil {
		fmt.Fprintf(os.Stderr, "Exec of cmd: %s timed out\n", parts)
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Exec of cmd: %s failed with err: '%s'\n", parts, err.Error())
	}
}

func askForSudoPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Please enter remote sudo password: ")
	stdin := int(syscall.Stdin)
	state, err := terminal.GetState(stdin)
	if err != nil {
		return "", err
	}
	utils.AddFinalizer(func() {
		terminal.Restore(stdin, state)
	})
	bytePassword, err := terminal.ReadPassword(stdin)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)
	return string(bytePassword), nil
}

func writeSudoPassword(cmd *exec.Cmd, sudoPasswd string) (err error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	io.WriteString(stdin, sudoPasswd+"\n")
	stdin.Close()

	return nil
}

func (ctx *SSHContext) ActivateConfiguration(host Host, configuration string, action string) error {

	if action == "switch" || action == "boot" {
		cmd, err := ctx.SudoCmd(host, "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", configuration)
		if err != nil {
			return err
		}

		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	args := []string{filepath.Join(configuration, "bin/switch-to-configuration"), action}

	var (
		cmd *exec.Cmd
		err error
	)
	cmd, err = ctx.SudoCmd(host, args...)
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return errors.New("Error while activating new configuration.")
	}

	return nil
}

func (sshCtx *SSHContext) GetBootID(host Host) (string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	cmd, err := sshCtx.CmdContext(ctx, host, "cat", "/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (ctx *SSHContext) MakeTempFile(host Host) (path string, err error) {
	cmd, _ := ctx.Cmd(host, "mktemp")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s (%s):\nCouldn't create temporary file using mktemp\n\nOriginal error:\n%s",
			host.GetName(), host.GetTargetHost(), stderr.String(),
		)
		return "", errors.New(errorMessage)
	}

	tempFile := strings.TrimSpace(stdout.String())

	return tempFile, nil
}

func (ctx *SSHContext) UploadFile(host Host, source string, destination string) (err error) {
	c, parts := ctx.sshArgs(host, &FileTransfer{
		Source:      source,
		Destination: destination,
	})
	cmd := exec.Command(c, parts...)

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s (%s):\nCouldn't upload file: %s -> %s\n\nOriginal error:\n%s",
			host.GetName(), host.GetTargetHost(), source, destination, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx *SSHContext) MakeDirs(host Host, path string, parents bool, mode os.FileMode) (err error) {

	parts := make([]string, 0)
	parts = append(parts, "mkdir")
	if parents {
		parts = append(parts, "-p")
	}
	parts = append(parts, "-m")
	parts = append(parts, fmt.Sprintf("%o", mode.Perm()))
	parts = append(parts, path)

	cmd, err := ctx.SudoCmd(host, parts...)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"\tCouldn't make directories: %s, on remote host. Error: %s", path, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx *SSHContext) MoveFile(host Host, source string, destination string) (err error) {
	cmd, err := ctx.SudoCmd(host, "mv", source, destination)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"\tCouldn't move file: %s -> %s:\n\t%s", source, destination, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx *SSHContext) SetOwner(host Host, path string, user string, group string) (err error) {
	cmd, err := ctx.SudoCmd(host, "chown", user+":"+group, path)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"\tCouldn't chown file: %s:\n\t%s", path, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx *SSHContext) SetPermissions(host Host, path string, permissions string) (err error) {
	cmd, err := ctx.SudoCmd(host, "chmod", permissions, path)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"\tCouldn't chmod file: %s:\n\t%s", path, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}
