
## Nix shell

All commands mentioned below is available in the nix-shell, if you run `nix-shell` with working dir = project root.

## Go dependencies

Run `make-deps` in order to:

1. (re-)install pinned dependencies in vendor-dir.
2. (re-)generate nix-packaging/deps.nix.

The former is done to support local dev, since IDE's often auto-import dependencies residing in /vendor.
The latter is used by the Nix go-builder.

Gopkg.toml specifies at which branch/tag each dependency is requested to be at.
Gopkg.lock specifies a concrete revision each dependency is pinned at.

If you want to bump dependencies to newest commit, run `make-deps update`, this will change Gopkg.lock and nix-packaging/deps.nix, both of which have to be git-committed.

If you make larger changes to the code base, you can delete both Gopkg.toml and Gopkg.lock and run `dep init` followed by `dep ensure` to create a fresh set of dependency tracking files. **don't forget to test** afterwards.

## Building the project with pinned dependencies

$ `nix-shell`

$ `make-build`

After successful build, `make-build` automatically invokes `make-env` to install the morph bin on the PATH of your nix-shell instance. Subsequently, it sources the morph bash-completion script to allow for completion of morph cli args and flags.
