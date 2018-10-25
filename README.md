# Morph

Morph is a tool for managing existing NixOS hosts -- basically a fancy wrapper around `nix-build`, `nix copy`, `nix-env`, `/nix/store/.../bin/switch-to-configuration`, `scp` and more.

The input is hosts defined in a manner similar to what is known from NixOps.


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


## About the project

We needed a tool for managing our NixOS servers, and ended up writing one ourself. This is it. We use it on a daily basis to build and deploy our NixOS fleet, and when we need a feature we add it.

Morph is by no means done. The CLI UI might (and probably will) change once in a while.
The code is written by humans with an itch to scratch, and we're discussing a complete rewrite (so feel free to complain about the source code since we don't like it either).
It probably wont accidentally switch your local machine, so you should totally try it out, but do consider pinning to a specific git revision.
