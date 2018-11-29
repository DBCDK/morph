# Completely stripped down version of nixops' evaluator
{ networkExpr }:

let
  network = import networkExpr;
  pkgs    = network.network.pkgs;
  lib     = pkgs.lib;
in
  with lib;

rec {
  # Compute the definitions of the machines.
  nodes =
    listToAttrs (map (machineName:
      let
        # Get the configuration of this machine from each network
        # expression, attaching _file attributes so the NixOS module
        # system can give sensible error messages.
        modules = [ { imports = [ network.${machineName} ]; } { inherit (network) _file; } ];
      in
      { name = machineName;
        value = import "${toString pkgs.path}/nixos/lib/eval-config.nix" {
          modules =
            modules ++
            [ ({config, ... }: {
                key = "deploy-stuff";
                imports = [ ./options.nix ];
                # Provide a default hostname and deployment target equal
                # to the attribute name of the machine in the model.
                networking.hostName = mkOverride 900 machineName;
                deployment.targetHost = mkOverride 900 machineName;
                nixpkgs.pkgs = mkDefault (import (toString pkgs.path) {
                  inherit (config.nixpkgs) config overlays localSystem crossSystem;
                });
              })
            ];
          extraArgs = { inherit nodes ; name = machineName; };
        };
      }
    ) (attrNames (removeAttrs network [ "network" "defaults" "resources" "require" "_file" ])));


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
      nodes' = nodes;
    in rec {

    machines =
      flip mapAttrs nodes (n: v': let v = scrubOptionValue v'; in
        { inherit (v.config.deployment) targetHost secrets healthChecks buildOnly;
          name = n;
          nixosRelease = v.config.system.nixos.release or (removeSuffix v.config.system.nixos.version.suffix v.config.system.nixos.version);
        }
      );

    machineList = (map (key: getAttr key machines) (attrNames machines));
    network = network'.network or {};
  };

  # Phase 2: build complete machine configurations.
  machines = { names }:
    let nodes' = filterAttrs (n: v: elem n names) nodes; in
    pkgs.runCommand "morph"
      { preferLocalBuild = true; }
      ''
        mkdir -p $out
        ${toString (attrValues (mapAttrs (n: v: ''
          ln -s ${v.config.system.build.toplevel} $out/${n}
          ln -s ${v.config.system.build.toplevel.drvPath} $out/${n}.drv
        '') nodes'))}
      '';

  # Function needed to calculate the nixops arguments. This should work even when arguments
  # are not set yet, so we fake arguments to be able to evaluate the require attribute of
  # the nixops network expressions.

  dummyArgs = f: builtins.listToAttrs (map (a: lib.nameValuePair a false) (builtins.attrNames (builtins.functionArgs f)));

  getNixOpsExprs = l: lib.unique (lib.flatten (map getRequires l));

  getRequires = f:
    let
      nixopsExpr = import f;
      requires =
        if builtins.isFunction nixopsExpr then
          ((nixopsExpr (dummyArgs nixopsExpr)).require or [])
        else
          (nixopsExpr.require or []);
    in
      [ f ] ++ map getRequires requires;

  fileToArgs = f:
    let
      nixopsExpr = import f;
    in
      if builtins.isFunction nixopsExpr then
        map (a: { "${a}" = builtins.toString f; } ) (builtins.attrNames (builtins.functionArgs nixopsExpr))
      else [];

  getNixOpsArgs = fs: lib.zipAttrs (lib.unique (lib.concatMap fileToArgs (getNixOpsExprs fs)));
}
