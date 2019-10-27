# morph
[![Build Status](https://travis-ci.org/DBCDK/morph.svg?branch=master)](https://travis-ci.org/DBCDK/morph)

Morph is a tool for managing existing NixOS hosts - basically a fancy wrapper around `nix-build`, `nix copy`, `nix-env`, `/nix/store/.../bin/switch-to-configuration`, `scp` and more.
Morph supports updating multiple hosts in a row, and with support for health checks makes it fairly safe to do so.


## Notable features

* multi host support
* health checks
* no state


## Installation and prerequisites

Morph requires `nix` (at least v2), `ssh` and `scp` to be available on `$PATH`.
It should work on any modern Linux distribution, but NixOS is the only one we test on.

Pre-built binaries are not provided, since we install morph through an overlay.

The easiest way to get morph up and running is to fork this repository and run `nix-shell --command make-build`, which should result in a store path containing the morph binary.
Consider checking out a specific tag, or at least pin the version of morph you're using somehow.


## Using morph

All commands support a `--help` flag; `morph --help` as of v1.0.0:
```
$ morph --help
usage: morph [<flags>] <command> [<args> ...]

NixOS host manager

Flags:
  --help     Show context-sensitive help (also try --help-long and --help-man).
  --version  Show application version.
  --dry-run  Don't do anything, just eval and print changes

Commands:
  help [<command>...]
    Show help.

  build [<flags>] <deployment>
    Evaluate and build deployment configuration to the local Nix store

  push [<flags>] <deployment>
    Build and transfer items from the local Nix store to target machines

  deploy [<flags>] <deployment> <switch-action>
    Build, push and activate new configuration on machines according to switch-action

  check-health [<flags>] <deployment>
    Run health checks

  upload-secrets [<flags>] <deployment>
    Upload secrets

  exec [<flags>] <deployment> <command>...
    Execute arbitrary commands on machines
```

Notably, `morph deploy` requires a `<switch-action>`.
The switch-action must be one of `dry-activate`, `test`, `switch` or `boot` corresponding to `nixos-rebuild` arguments of the same name.
Refer to the [NixOS manual](https://nixos.org/nixos/manual/index.html#sec-changing-config) for a detailed description of switch-actions.

For help on this and other commands, run `morph <cmd> --help`.

Example deployments can be found in the `examples` directory, and built as follows:
```
$ morph build examples/simple.nix
Selected 2/2 hosts (name filter:-0, limits:-0):
	  0: db01.example.com (secrets: 0, health checks: 0)
	  1: web01.example.com (secrets: 0, health checks: 0)

<probably lots of nix-build output>

/nix/store/grvny5ga2i6jdxjjbh2ipdz7h50swi1n-morph
nix result path:
/nix/store/grvny5ga2i6jdxjjbh2ipdz7h50swi1n-morph
```

The result path is written twice, which is a bit silly, but the reason is that only the result path is written to stdout, and everything else (including `nix-build` output) is redirected to stderr.
This makes it easy to use morph for scripting, e.g. if one want to build using morph and then `nix copy` the result path somewhere else.

Note that `examples/simple.nix` contain two different hosts definitions, and a lot of copy paste.
All the usual nix tricks can of course be used to avoid duplication.

Hosts can be deployed with the `deploy` command as follows:
`morph deploy examples/simple.nix` (this will fail without modifying `examples/simple.nix`).


### Selecting/filtering hosts to build and deploy

All hosts defined in a deployment file is returned to morph as a list of hosts, which can be manipulated with the following flags:

- `--on glob` can be used to select hosts by name, with support for glob patterns
- `--limit n` puts an upper limit on the number of hosts
- `--skip n` ignore the first `n` hosts
- `--every n` selects every n'th host, useful for e.g. selecting all even (or odd) numbered hosts

(all relevant commands should already support these flags.)

The ordering currently can't be changed, but should be deterministic because of nix.

Most commands output a header like this:
```
Selected 4/17 hosts (name filter:-6, limits:-7):
	  0: foo-p02 (secrets: 0, health checks: 1)
	  1: foo-p05 (secrets: 0, health checks: 1)
	  2: foo-p08 (secrets: 0, health checks: 1)
	  3: foo-p11 (secrets: 0, health checks: 1)
```

The output is pretty self explanatory, except probably for the last bit of the first line.
`name filter` shows the change in number of hosts after glob matching on the hosts name, and `limits` shows the change after applying `--limit`, `--skip` and `--every`.

### Environment Variables

Morph supports the following (optional) environment variables:

- `SSH_IDENTITY_FILE` the (local) path to the SSH private key file that should be used
- `SSH_USER` specifies the user that should be used to connect to the remote system
- `SSH_SKIP_HOST_KEY_CHECK` if set disables host key verification

### Secrets

Files can be uploaded without ever ending up in the nix store, by specifying each file as a secret. This will use scp for copying a local file to the remote host.

See `examples/secrets.nix` or the type definitions in `data/options.nix`.

To upload secrets, use the `morph upload-secrets` subcommand, or pass `--upload-secrets` to `morph deploy`.

*Note:*
Morph will automatically create directories parent to `secret.Destination` if they don't exist.
New dirs will be owned by root:root and have mode 755 (drwxr-xr-x).
Automatic directory creation can be disabled by setting `secret.mkDirs = false`.


### Health checks

Morph has support for two types of health checks:

* command based health checks, which are run on the target host (success defined as exit code == 0)
* HTTP based health checks, which are run from the host Morph is running on (success defined as HTTP response codes in the 2xx range)

See `examples/healthchecks.nix` for an example.

There are no guarantees about the order health checks are run in, so if you need something complex you should write a script for it (e.g. using `pkgs.writeScript`).
Health checks will be repeated until success, and the interval can be configured with the `period` option (see `data/options.nix` for details).

It is currently possible to have expressions like `"test \"$(systemctl list-units --failed --no-legend --no-pager |wc -l)\" -eq 0"` (count number of failed systemd units, fail if non-zero) as the first argument in a cmd-healthcheck. This works, but is discouraged, and might break at any time.

### Advanced configuration

**nix.conf-options:** The "network"-attrset supports a sub-attrset named "nixConfig". Options configured here will pass `--option <name> <value>` to all nix commands.
Note: these options apply to an entire deployment and are *not* configurable on per-host basis.
The default is an empty set, meaning that the nix configuration is inherited from the build environment. See `man nix.conf`.

**special deployment options:**

(per-host granularity)

`buildOnly` makes morph skip the "push" and "switch" steps for the given host, even if "morph deploy" or "morph push" is executed. (default: false)

`substituteOnDestination` Sets the `--substitute-on-destination` flag on nix copy, allowing for the deployment target to use substitutes. See `nix copy --help`. (default: false)


Example usage of `nixConfig` and deployment module options:
```
network = {
    nixConfig = {
        "extra-sandbox-paths" = "/foo/bar";
    };
};

machine1 = { ... }: {
    deployment.buildOnly = true;
};

machine2 = { ... }: {
    deployment.substituteOnDestination = true;
};
```


## Hacking morph

All commands mentioned below is available in the nix-shell, if you run `nix-shell` with working dir = project root.


### Go dependency management

Run `make-deps` in order to:

1. (re-)install pinned dependencies in vendor-dir.
2. (re-)generate nix-packaging/deps.nix.

The former is done to support local dev, since IDE's often auto-import dependencies residing in /vendor.
The latter is used by the Nix go-builder.

Gopkg.toml specifies at which branch/tag each dependency is requested to be at.
Gopkg.lock specifies a concrete revision each dependency is pinned at.

If you want to bump dependencies to newest commit, run `make-deps update`, this will change Gopkg.lock and nix-packaging/deps.nix, both of which have to be git-committed.

If you make larger changes to the code base, you can delete both Gopkg.toml and Gopkg.lock and run `dep init` followed by `dep ensure` to create a fresh set of dependency tracking files. **don't forget to test** afterwards.

### Building the project with pinned dependencies

$ `nix-shell`

$ `make-build`

After successful build, `make-build` automatically invokes `make-env` to install the morph bin on the PATH of your nix-shell instance. Subsequently, it sources the morph bash-completion script to allow for completion of morph cli args and flags.


## About the project

We needed a tool for managing our NixOS servers, and ended up writing one ourself. This is it. We use it on a daily basis to build and deploy our NixOS fleet, and when we need a feature we add it.

Morph is by no means done. The CLI UI might (and probably will) change once in a while.
The code is written by humans with an itch to scratch, and we're discussing a complete rewrite (so feel free to complain about the source code since we don't like it either).
It probably wont accidentally switch your local machine, so you should totally try it out, but do consider pinning to a specific git revision.
