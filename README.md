
## Nix shell

All commands mentioned below is available in the nix-shell, if you run `nix-shell` with working dir = project root.

## Go dependencies

Run `make dependencies` to (re-)install pinned dependencies in vendor-dir

Gopkg.toml specifies at which branch/tag each dependency is requested to be at.
Gopkg.lock specifies a concrete revision each dependency is pinned at.

If you want to bump dependencies to newest commit, run `dep ensure -v -update`, this will change Gopkg.lock, which has to be git-committed.

If you make larger changes to the code base, you can delete both Gopkg.toml and Gopkg.lock and run `dep init` followed by `dep ensure` to create a fresh set of dependency tracking files. **don't forget to test** afterwards.

## Assets

Run `make assets` after updating files from data/

## Building the project with pinned dependencies

$ `nix-shell`

$ `make`

*Your GOPATH must be set in your local environment, however /vendor is used exclusively for dependency resolution.*


## Building a nix derivation

$ `nix-shell`

$ `make derivation`

*Produces "default.nix" and "deps.nix" which can be copied to the deployments repo*
