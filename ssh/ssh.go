package ssh

import (
	"errors"
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func sshSudoCmd(host nix.Host, sudoPasswd string, parts ...string) (cmd *exec.Cmd, err error) {
	askPasswd := len(sudoPasswd) > 0
	cmdArgs := []string{nix.GetHostname(host), "sudo"}
	if askPasswd {
		cmdArgs = append(cmdArgs, "-S")
	} else {
		// no password supplied; request non-interactive sudo, which will fail with an error if a password was required
		cmdArgs = append(cmdArgs, "-n")
	}

	cmdArgs = append(cmdArgs, "-p", "''", "-k", "--")
	cmdArgs = append(cmdArgs, parts...)
	cmd = exec.Command("ssh", cmdArgs...)
	if askPasswd {
		err := writeSudoPassword(cmd, sudoPasswd)
		if err != nil {
			return nil, err
		}
	}

	return
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

func ActivateConfiguration(host nix.Host, configuration string, action string, sudoPasswd string) error {

	if action == "switch" || action == "boot" {
		cmd, err := sshSudoCmd(host, sudoPasswd, "nix-env", "--profile", "/nix/var/nix/profiles/system", "--set", configuration)
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

	cmd, err := sshSudoCmd(host, sudoPasswd, filepath.Join(configuration, "bin/switch-to-configuration"), action)
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

func MakeTempFile(host nix.Host) (path string, err error) {
	cmd := exec.Command(
		"ssh", nix.GetHostname(host), "mktemp",
	)

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

func UploadFile(host nix.Host, source string, destination string) (err error) {
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

func MoveFile(host nix.Host, sudoPasswd string, source string, destination string) (err error) {
	cmd, err := sshSudoCmd(host, sudoPasswd, "mv", source, destination)
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

func SetOwner(host nix.Host, sudoPasswd string, path string, user string, group string) (err error) {
	cmd, err := sshSudoCmd(host, sudoPasswd, "chown", user+"."+group, path)
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

func SetPermissions(host nix.Host, sudoPasswd string, path string, permissions string) (err error) {
	cmd, err := sshSudoCmd(host, sudoPasswd, "chmod", permissions, path)
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
