package assets

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

const Friendly string = "morph"

func Setup() (assetRoot string, err error) {
	assetRoot, err = ioutil.TempDir("", "morph-")
	if err != nil {
		return "", err
	}

	assetFriendlyRoot := filepath.Join(assetRoot, Friendly)
	err = os.Mkdir(assetFriendlyRoot, 0755)
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

	evalMachinesPath := filepath.Join(assetFriendlyRoot, "eval-machines.nix")
	optionsPath := filepath.Join(assetFriendlyRoot, "options.nix")
	ioutil.WriteFile(evalMachinesPath, evalMachinesData, 0644)
	ioutil.WriteFile(optionsPath, optionsData, 0644)

	return
}

func Teardown(assetRoot string) (err error) {
	assetFriendlyRoot := filepath.Join(assetRoot, Friendly)

	err = os.Remove(filepath.Join(assetFriendlyRoot, "eval-machines.nix"))
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(assetFriendlyRoot, "options.nix"))
	if err != nil {
		return err
	}

	err = os.Remove(assetFriendlyRoot)
	if err != nil {
		return err
	}

	err = os.Remove(assetRoot)
	if err != nil {
		return err
	}

	return nil
}
