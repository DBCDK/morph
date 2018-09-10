package ssh

import (
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type Context interface {
	ActivateConfiguration(host nix.Host, configuration string, action string) error
	MakeTempFile(host nix.Host) (path string, err error)
	UploadFile(host nix.Host, source string, destination string) error
	SetOwner(host nix.Host, path string, user string, group string) error
	SetPermissions(host nix.Host, path string, permissions string) error
	MoveFile(host nix.Host, source string, destination string) error

	Cmd(host nix.Host, parts ...string) (*exec.Cmd, error)
	SudoCmd(host nix.Host, parts ...string) (*exec.Cmd, error)

	cmd(cmdArgs []string) (*exec.Cmd, error)
}

type SSHContext struct {
	sudoPassword       string
	AskForSudoPassword bool
}

func (ctx SSHContext) Cmd(host nix.Host, parts ...string) (*exec.Cmd, error) {

	if parts[0] == "sudo" {
		ctx.SudoCmd(host, parts...)
	}

	cmdArgs := []string{nix.GetHostname(host)}
	cmdArgs = append(cmdArgs, parts...)

	return ctx.cmd(cmdArgs)
}

func (ctx SSHContext) SudoCmd(host nix.Host, parts ...string) (*exec.Cmd, error) {

	// ask for password if not done already
	if ctx.AskForSudoPassword && ctx.sudoPassword == "" {
		ctx.sudoPassword = askForSudoPassword()
	}

	cmdArgs := []string{nix.GetHostname(host)}
	// prepend "sudo" to command if not already supplied
	if parts[0] != "sudo" {
		cmdArgs = append(cmdArgs, "sudo")
	}

	if ctx.sudoPassword != "" {
		cmdArgs = append(cmdArgs, "-S")
	} else {
		// no password supplied; request non-interactive sudo, which will fail with an error if a password was required
		cmdArgs = append(cmdArgs, "-n")
	}

	cmdArgs = append(cmdArgs, "-p", "''", "-k", "--")
	cmdArgs = append(cmdArgs, parts...)

	return ctx.cmd(cmdArgs)
}

func (ctx SSHContext) cmd(cmdArgs []string) (*exec.Cmd, error) {
	command := exec.Command("ssh", cmdArgs...)
	if ctx.sudoPassword != "" {
		err := writeSudoPassword(command, ctx.sudoPassword)
		if err != nil {
			return nil, err
		}
	}
	return command, nil
}

func askForSudoPassword() string {
	fmt.Print("Please enter remote sudo password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		panic(err)
	}
	return string(bytePassword)
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

func (ctx SSHContext) ActivateConfiguration(host nix.Host, configuration string, action string) error {

	if action == "switch" || action == "boot" {
		cmd, err := ctx.SudoCmd(host, "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", configuration)
		if err != nil {
			return err
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	args := []string{ filepath.Join(configuration, "bin/switch-to-configuration"), action }

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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return errors.New("Error while activating new configuration.")
	}

	return nil
}

func (ctx SSHContext) MakeTempFile(host nix.Host) (path string, err error) {
	cmd, _ := ctx.Cmd(host, "mktemp")

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't create temporary file using mktemp\n\nOriginal error:\n%s",
			nix.GetHostname(host), string(data),
		)
		return "", errors.New(errorMessage)
	}

	tempFile := strings.TrimSpace(string(data))

	return tempFile, nil
}

func (ctx SSHContext) UploadFile(host nix.Host, source string, destination string) (err error) {
	destinationAndHost := nix.GetHostname(host) + ":" + destination
	cmd := exec.Command(
		"scp", source, destinationAndHost,
	)

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't upload file: %s -> %s\n\nOriginal error:\n%s",
			nix.GetHostname(host), source, destinationAndHost, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx SSHContext) MoveFile(host nix.Host, source string, destination string) (err error) {
	cmd, err := ctx.SudoCmd(host, "mv", source, destination)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't move file: %s -> %s\n\nOriginal error:\n%s",
			nix.GetHostname(host), source, destination, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx SSHContext) SetOwner(host nix.Host, path string, user string, group string) (err error) {
	cmd, err := ctx.SudoCmd(host, "chown", user+"."+group, path)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't chown file: %s\n\nOriginal error:\n%s",
			nix.GetHostname(host), path, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}

func (ctx SSHContext) SetPermissions(host nix.Host, path string, permissions string) (err error) {
	cmd, err := ctx.SudoCmd(host, "chmod", permissions, path)
	if err != nil {
		return err
	}

	data, err := cmd.CombinedOutput()
	if err != nil {
		errorMessage := fmt.Sprintf(
			"Error on remote host %s:\nCouldn't chmod file: %s\n\nOriginal error:\n%s",
			nix.GetHostname(host), path, string(data),
		)
		return errors.New(errorMessage)
	}

	return nil
}
