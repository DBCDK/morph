# Completely stripped down version of nixops' evaluator
{
  networkExpr ? null,
  network ? import networkExpr,
}:

let
  nwPkgs = network.network.pkgs or { };
  lib = network.network.lib or nwPkgs.lib or (import <nixpkgs/lib>);
  evalConfig =
    network.network.evalConfig or ((nwPkgs.path or <nixpkgs>) + "/nixos/lib/eval-config.nix");
  specialArgs = network.network.specialArgs or { };
  runCommand = network.network.runCommand or nwPkgs.runCommand or (import <nixpkgs> { }).runCommand;
in
with lib;

let
  defaults = network.defaults or { };

  modules =
    {
      machineName,
      nodes,
      check,
    }:
    [
      # Get the configuration of this machine from each network
      # expression, attaching _file attributes so the NixOS module
      # system can give sensible error messages.
      { imports = [ network.${machineName} ]; }

      defaults

      (
        { lib, ... }:
        {
          key = "deploy-stuff";
          imports = [ ./options.nix ];
          # Make documentation builds deterministic, even with our
          # tempdir module imports.
          documentation.nixos.extraModuleSources = [ ../. ];
          # Provide a default hostname and deployment target equal
          # to the attribute name of the machine in the model.
          networking.hostName = lib.mkDefault machineName;
          deployment.targetHost = lib.mkDefault machineName;

          # If network.pkgs is set, mkDefault nixpkgs.pkgs
          nixpkgs.pkgs = lib.mkIf (nwPkgs != { }) (lib.mkDefault nwPkgs);

          # Avoid the deprecated evalConfig arguments by
          # setting them here instead.
          _module = {
            args = {
              name = machineName;
              inherit nodes;
            };
            inherit check;
          };
        }
      )
    ]
    ++ optional (network ? _file) { inherit (network) _file; };

  machineNames = attrNames (
    removeAttrs network [
      "network"
      "defaults"
      "resources"
      "require"
      "_file"
    ]
  );

in
rec {
  # Unchecked configuration of all machines.
  # Using unchecked config evaluation allows each machine to access other machines
  # configuration without recursing as full evaluation is prevented
  uncheckedNodes = listToAttrs (
    map (machineName: {
      name = machineName;
      value = import evalConfig {
        inherit specialArgs;
        # Force decide system in module system
        system = null;
        modules = modules {
          inherit machineName;
          check = false;
          nodes = uncheckedNodes;
        };
      };
    }) machineNames
  );

  # Compute the definitions of the machines.
  nodes = listToAttrs (
    map (machineName: {
      name = machineName;
      value = import evalConfig {
        inherit specialArgs;
        # Force decide system in module system
        system = null;
        modules = modules {
          inherit machineName;
          check = true;
          nodes = uncheckedNodes;
        };
      };
    }) machineNames
  );

  deploymentInfoModule = {
    deployment = {
      name = deploymentName;
      arguments = args;
      inherit uuid;
    };
  };

  # Phase 1: evaluate only the deployment attributes.
  info =
    let
      network' = network;
    in
    rec {

      machines = flip mapAttrs nodes (
        n: v':
        let
          v = scrubOptionValue v';
        in
        {
          inherit (v.config.deployment)
            targetHost
            targetPort
            targetUser
            secrets
            preDeployChecks
            healthChecks
            buildOnly
            substituteOnDestination
            tags
            ;
          name = n;
          nixosRelease =
            v.config.system.nixos.release
              or (removeSuffix v.config.system.nixos.version.suffix v.config.system.nixos.version);
          nixConfig = mapAttrs (
            n: v: if builtins.isString v then v else throw "nix option '${n}' must have a string typed value"
          ) (network'.network.nixConfig or { });
        }
      );

      machineList = map (key: getAttr key machines) (attrNames machines);
      network = network'.network or { };
      deployment = {
        hosts = machineList;
        meta = {
          description = network.description or "";
          ordering = network.ordering or { };
        };
      };

      buildShell = network.buildShell.drvPath or null;
    };

  # Phase 2: build complete machine configurations.
  machines =
    {
      argsFile,
      buildTargets ? null,
    }:
    let
      fileArgs = builtins.fromJSON (builtins.readFile argsFile);
      nodes' = filterAttrs (n: _v: elem n fileArgs.Names) nodes;
    in
    runCommand "morph" { preferLocalBuild = true; } (
      if buildTargets == null then
        ''
          mkdir -p $out
          ${toString (
            mapAttrsToList (nodeName: nodeDef: ''
              ln -s ${nodeDef.config.system.build.toplevel} $out/${nodeName}
            '') nodes'
          )}
        ''
      else
        ''
          mkdir -p $out
          ${toString (
            mapAttrsToList (nodeName: nodeDef: ''
              mkdir -p $out/${nodeName}
              ${toString (
                mapAttrsToList (buildName: buildFn: ''
                  ln -s ${buildFn nodeDef} $out/${nodeName}/${buildName}
                '') buildTargets
              )}
            '') nodes'
          )}
        ''
    );

}
