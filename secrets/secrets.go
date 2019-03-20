package secrets

import (
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
	"os"
	"path/filepath"
)

type SecretError struct {
	Err   error
	Fatal bool
}

func wrap(err error) *SecretError {
	return &SecretError{
		Err:   err,
		Fatal: true,
	}
}

func wrapNonFatal(err error) *SecretError {
	return &SecretError{
		Err:   err,
		Fatal: false,
	}
}

func (e SecretError) Error() string {
	return e.Err.Error()
}

func GetSecretSize(secret Secret, deploymentWD string) (size int64, err error) {
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

func UploadSecret(ctx ssh.Context, host ssh.Host, secret Secret, deploymentWD string) *SecretError {
	var partialErr *SecretError

	tempPath, err := ctx.MakeTempFile(host)
	if err != nil {
		return wrap(err)
	}

	if secret.MkDirs {
		if err := ctx.MakeDirs(host, filepath.Dir(secret.Destination), true, 0755); err != nil {
			return wrap(err)
		}
	}

	err = ctx.UploadFile(host, utils.GetAbsPathRelativeTo(secret.Source, deploymentWD), tempPath)
	if err != nil {
		return wrap(err)
	}

	err = ctx.MoveFile(host, tempPath, secret.Destination)
	if err != nil {
		return wrap(err)
	}

	err = ctx.SetOwner(host, secret.Destination, secret.Owner.User, secret.Owner.Group)
	if err != nil {
		partialErr = wrapNonFatal(err)
	}

	err = ctx.SetPermissions(host, secret.Destination, secret.Permissions)
	if err != nil {
		partialErr = wrapNonFatal(err)
	}

	return partialErr
}
