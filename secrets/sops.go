package secrets

import (
	"os/exec"
	"strings"

	"github.com/dbcdk/morph/utils"
)

func ExtractSopsSecret(secretPath, deploymentWD, outFileName string) error {
	// sops secret pseudo URN format:
	//   sops:<file-path>
	//   sops:<file-path>?<extract-args>
	// where <extract-args> are passed to sops with --extract
	// For example: sops:/secrets/mysecret.enc.yaml?["mypassword"] would
	// open an encrypted yaml file /secrets/mysecret.enc.yaml, extract a top level
	// key named mypassword, and upload that.
	stripped := strings.TrimPrefix(secretPath, "sops:")
	split := strings.SplitN(stripped, "?", 2)
	sopsFileName := utils.GetAbsPathRelativeTo(split[0], deploymentWD)
	var extractArgs = []string{}
	if len(split) == 2 {
		extractArgs = []string{"--extract", split[1]}
	}
	args := []string{"-d"}
	args = append(args, extractArgs...)
	args = append(args, []string{"--output", outFileName, sopsFileName}...)

	err := exec.Command("sops", args...).Run()
	if err != nil {
		return err
	}
	return nil
}
