package secrets

import (
	"git-platform.dbc.dk/platform/morph/nix"
	"git-platform.dbc.dk/platform/morph/ssh"
	"git-platform.dbc.dk/platform/morph/utils"
	"os"
)

func GetSecretSize(secret nix.Secret, deploymentWD string) (size int64, err error) {
	fh, err := os.Open(utils.GetAbsPathRelativeTo(secret.Source, deploymentWD))
	if err != nil {
		return size, err
	}

	fStats, err := fh.Stat()
	if err != nil {
		return size, err
	}

	return fStats.Size(), nil
}

func UploadSecret(ctx ssh.Context, host nix.Host, secret nix.Secret, deploymentWD string) (err error) {
	tempPath, err := ctx.MakeTempFile(host)
	if err != nil {
		return err
	}

	err = ctx.UploadFile(host, utils.GetAbsPathRelativeTo(secret.Source, deploymentWD), tempPath)
	if err != nil {
		return err
	}

	err = ctx.MoveFile(host, tempPath, secret.Destination)
	if err != nil {
		return err
	}

	err = ctx.SetOwner(host, secret.Destination, secret.Owner.User, secret.Owner.Group)
	if err != nil {
		return err
	}

	err = ctx.SetPermissions(host, secret.Destination, secret.Permissions)
	if err != nil {
		return nil
	}

	return nil
}
