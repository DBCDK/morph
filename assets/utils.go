package assets

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func Setup() (assetRoot string, err error) {
	assetRoot, err = ioutil.TempDir("", "morph-")
	if err != nil {
		return "", err
	}

	evalMachinesData, err := Asset("data/eval-machines.nix")
	if err != nil {
		return "", err
	}

	optionsData, err := Asset("data/options.nix")
	if err != nil {
		return "", err
	}

	evalMachinesPath := filepath.Join(assetRoot, "eval-machines.nix")
	optionsPath := filepath.Join(assetRoot, "options.nix")
	ioutil.WriteFile(evalMachinesPath, evalMachinesData, 0644)
	ioutil.WriteFile(optionsPath, optionsData, 0644)

	return
}

func Teardown(assetRoot string) (err error) {
	err = os.Remove(filepath.Join(assetRoot, "eval-machines.nix"))
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(assetRoot, "options.nix"))
	if err != nil {
		return err
	}

	err = os.Remove(assetRoot)
	if err != nil {
		return err
	}

	return nil
}
