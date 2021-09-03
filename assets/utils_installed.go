//go:build installed_data
// +build installed_data

package assets

const Friendly string = ""
var root string

func Setup() (assetRoot string, err error) {
	assetRoot = root

	return
}

func Teardown(assetRoot string) (err error) {
	return nil
}
