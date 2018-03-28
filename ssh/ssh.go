package ssh

import (
	"fmt"
	"git-platform.dbc.dk/platform/morph/nix"
	"io"
	"os/exec"
)

var sudoCMD = "sudo -S -p '' -k "

func ActivateConfiguration(host nix.Host, configuration string, action string, sudoPasswd string) error {

	cmdStr := configuration + "/bin/switch-to-configuration " + action
	if action != "dry-activate" {
		cmdStr = sudoCMD + cmdStr
	}

	cmd := exec.Command(
		"ssh", host.TargetHost,
		cmdStr,
	)

	// Write sudo pass on ssh process stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	io.WriteString(stdin, sudoPasswd+"\n")
	defer stdin.Close()

	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))

	if err != nil {
		return err
	}

	return nil
}
