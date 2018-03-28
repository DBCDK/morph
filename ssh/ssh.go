package ssh

import (
	"os/exec"
	"git-platform.dbc.dk/platform/morph/nix"
	"fmt"
)

var sudoCMD = "sudo -S -p '' -k "

func ActivateConfiguration(host nix.Host, configuration string, action string, sudoPasswd string) error {

	cmdStr := configuration + "/bin/switch-to-configuration " + action
	if action != "dry-activate" {
		cmdStr = "echo \"" + sudoPasswd + "\" |" + sudoCMD + cmdStr
	}

	cmd := exec.Command(
		"ssh", host.TargetHost,
		cmdStr,
	)

	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))

	if err != nil {
		return err
	}

	return nil
}
