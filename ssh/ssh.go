package ssh

import (
	"errors"
	"fmt"
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
	GetTargetHost() string
}

type SSHContext struct {
	sudoPassword       string
	AskForSudoPassword bool
	Username           string
	IdentityFile       string
}

func (ctx *SSHContext) Cmd(host Host, parts ...string) (*exec.Cmd, error) {

	var err error
	if parts, err = valCommand(parts); err != nil {
		return nil, err
	}

	if parts[0] == "sudo" {
		return ctx.SudoCmd(host, parts...)
	}

	cmdArgs := ctx.initialSSHArgs(host)
	cmdArgs = append(cmdArgs, parts...)

	command := exec.Command("ssh", cmdArgs...)
	return command, nil
}

func (ctx *SSHContext) initialSSHArgs(host Host) []string {
	args := make([]string, 0)
	if ctx.IdentityFile != "" {
		args = append(args, "-i")
		args = append(args, ctx.IdentityFile)
	}
	if ctx.Username != "" {
		args = append(args, ctx.Username + "@" + host.GetTargetHost())
	} else {
		args = append(args, host.GetTargetHost())
	}

	return args
}

func (ctx *SSHContext) SudoCmd(host Host, parts ...string) (*exec.Cmd, error) {
	var err error
	if parts, err = valCommand(parts); err != nil {
		return nil, err
	}

	// ask for password if not done already
	if ctx.AskForSudoPassword && ctx.sudoPassword == "" {
		ctx.sudoPassword, err = askForSudoPassword()
		if err != nil {
			return nil, err
		}
	}

	cmdArgs := ctx.initialSSHArgs(host)

	// normalize sudo
	if parts[0] == "sudo" {
		parts = parts[1:]
	}
	cmdArgs = append(cmdArgs, "sudo")

	if ctx.sudoPassword != "" {
		cmdArgs = append(cmdArgs, "-S")
	} else {
		// no password supplied; request non-interactive sudo, which will fail with an error if a password was required
		cmdArgs = append(cmdArgs, "-n")
	}

	cmdArgs = append(cmdArgs, "-p", "''", "-k", "--")
	cmdArgs = append(cmdArgs, parts...)

	command := exec.Command("ssh", cmdArgs...)
	if ctx.sudoPassword != "" {
		err := writeSudoPassword(command, ctx.sudoPassword)
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

func (ctx *SSHContext) CmdInteractive(host Host, timeout int, parts ...string) {
	doneChan := make(chan bool)
	timeoutChan := make(chan bool)
	var cmd *exec.Cmd
	var err error
	if timeout > 0 {
		go func() {
			time.Sleep(time.Duration(timeout) * time.Second)
			timeoutChan <- true
		}()
	}
	go func() {
		cmd, err = ctx.Cmd(host, parts...)
		if err == nil {
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		}
		doneChan <- true

		if err != nil && !<-timeoutChan {
			fmt.Fprintf(os.Stderr, "Exec of cmd: %s failed with err: '%s'\n", parts, err.Error())
		}
	}()

	for {
		select {
		case <-timeoutChan:
			fmt.Fprintf(os.Stderr, "Exec of cmd: %s timed out\n", parts)
			cmd.Process.Kill()
			return
		case <-doneChan:
			return
		}
	}
}

func askForSudoPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Please enter remote sudo password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
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
	if action == "dry-activate" {
		cmd, err = ctx.Cmd(host, args...)
	} else {
		cmd, err = ctx.SudoCmd(host, args...)
	}
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

func (ctx *SSHContext) MakeTempFile(host Host) (path string, err error) {
	cmd, _ := ctx.Cmd(host, "mktemp")

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't create temporary file using mktemp\n\nOriginal error:\n%s",
			host.GetTargetHost(), string(data),
		)
		return "", errors.New(errorMessage)
	}

	tempFile := strings.TrimSpace(string(data))

	return tempFile, nil
}

func (ctx *SSHContext) UploadFile(host Host, source string, destination string) (err error) {
	destinationAndHost := host.GetTargetHost() + ":" + destination
	cmd := exec.Command(
		"scp", source, destinationAndHost,
	)

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't upload file: %s -> %s\n\nOriginal error:\n%s",
			host.GetTargetHost(), source, destinationAndHost, string(data),
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
