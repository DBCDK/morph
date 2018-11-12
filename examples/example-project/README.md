# Example project

This directory mimics a light-weight version of the directory structure that is used at DBC. It's probably quite a mouthful, but it's advised to look through it all before using it.

This is just one way to do things. Morph only cares about the output from the nix-file it is passed, not about the filelayout.
Feel free to use this as a starting point for your own adventures, but don't take it as gospel.

*Please read through any scripts before executing them!*


## Crash course

Copy this directory somewhere, and cd into the root of the directory.

The following scripts are useful:

* `bin/updatemorph`: fetch the latest version of morph from git
* `bin/updatepin 18.09`: update the pinned version of nixpkgs

Execute `nix-shell` in the root of the project to get a shell with the pinned version of morph installed, and bash completions sourced.
Once inside the shell, morph can be used like normal, e.g. `morph build hostgroups/webservers.nix` to build all hosts in the group.

Since morph currently can't work on more than one hostgroup file at the time (see issue #19), a convenience script is included to do just this:

* `./bin/morph-all-hosts deploy {} dry-activate` will run `morph deploy <hostgroup> dry-activate for each hostgroup in the hostgroups-directory
* `./bin/morph-all-hosts build {}` will build all hosts

`{}` is replaced with the path to each file, similar to when using `xargs`.


## The `hosts` directory

DBC use a single file to describe each host named after its UUID. Originally the files were nix-expressions, but since it's easier for other tools to work with json we started using that instead.

The purpose is to abstract everything host specific to these files, to avoid littering the rest of the code with hardware-specific details.

Our own host.json-files are much more complicated than what is seen here, including things such as definition of primary NIC's, network bonds, and more. One could also add a property to describe whether this is a virtual host, or not, which could then trigger installing VMware tools on the host.


## The `hostgroups` directory

Files in here describe systems of related hosts. Ideally nothing host-specific is included here, and hosts should be selected based on their UUID.
The examples in this directory is equivalent to the simple examples, but now rely on many other files, instead of being effectively stand-alone.