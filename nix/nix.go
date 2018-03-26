package nix

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type Host struct {
	Name         string
	NixosRelease string
	TargetHost   string
}

func GetMachines(evalMachines string, deploymentFile *os.File) (hosts []Host, err error) {
	cmd := exec.Command(
		"nix", "eval",
		"-f", evalMachines, "info.machineList",
		"--arg", "networkExpr", deploymentFile.Name(),
		"--json",
	)

	bytes, err := cmd.Output()
	if err != nil {
		return hosts, err
	}

	err = json.Unmarshal(bytes, &hosts)
	if err != nil {
		return hosts, err
	}

	return hosts, nil
}

func BuildMachines(evalMachines string, deploymentFile *os.File, hosts []Host) (path string, err error) {
	hostsArg := "["
	for _, host := range hosts {
		hostsArg += "\"" + host.TargetHost + "\" "
	}
	hostsArg += "]"

	// create tmp dir for result link
	tmpdir, err := ioutil.TempDir("", "morph-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpdir)

	resultLinkPath := filepath.Join(tmpdir, "result")

	cmd := exec.Command(
		"nix", "build",
		"-f", evalMachines, "machines",
		"--arg", "networkExpr", deploymentFile.Name(),
		"--arg", "names", hostsArg,
		"--out-link", resultLinkPath,
	)
	defer os.Remove(resultLinkPath)

	// show process output on attached stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	if err != nil {
		return "", err
	}

	resultPath, err := os.Readlink(resultLinkPath)
	if err != nil {
		return "", err
	}

	return resultPath, nil
}

func Push() {

}
