# Completely stripped down version of nixops' evaluator
{ networkExpr }:

let
  network      = import networkExpr;
  nwPkgs       = network.network.pkgs or {};
  lib          = network.network.lib or nwPkgs.lib or (import <nixpkgs/lib>);
  evalConfig   = network.network.evalConfig or "${nwPkgs.path or <nixpkgs>}/nixos/lib/eval-config.nix";
  runCommand   = network.network.runCommand or nwPkgs.runCommand or ((import <nixpkgs> {}).runCommand);
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
        value = import evalConfig {
          modules =
            modules ++
            [ ({ config, lib, options, ... }: {
                key = "deploy-stuff";
                imports = [ ./options.nix ];
                # Provide a default hostname and deployment target equal
                # to the attribute name of the machine in the model.
                networking.hostName = lib.mkDefault machineName;
                deployment.targetHost = lib.mkDefault machineName;

                # If network.pkgs is set, mkDefault nixpkgs.pkgs
                nixpkgs.pkgs = lib.mkIf (nwPkgs != {}) (lib.mkDefault nwPkgs);
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
        { inherit (v.config.deployment) targetHost targetUser secrets healthChecks buildOnly substituteOnDestination;
          name = n;
          nixosRelease = v.config.system.nixos.release or (removeSuffix v.config.system.nixos.version.suffix v.config.system.nixos.version);
          nixConfig = mapAttrs
            (n: v: if builtins.isString v then v else throw "nix option '${n}' must have a string typed value")
            (network'.network.nixConfig or {});
        }
      );

    machineList = (map (key: getAttr key machines) (attrNames machines));
    network = network'.network or {};
  };

  # Phase 2: build complete machine configurations.
  machines = { names, buildTargets ? null }:
    let nodes' = filterAttrs (n: v: elem n names) nodes; in
    runCommand "morph"
      { preferLocalBuild = true; }
      (if buildTargets == null
      then ''
        mkdir -p $out
        ${toString (mapAttrsToList (nodeName: nodeDef: ''
          ln -s ${nodeDef.config.system.build.toplevel} $out/${nodeName}
        '') nodes')}
      ''
      else ''
        mkdir -p $out
        ${toString (mapAttrsToList (nodeName: nodeDef: ''
          mkdir -p $out/${nodeName}
          ${toString (mapAttrsToList (buildName: buildFn: ''
            ln -s ${buildFn nodeDef} $out/${nodeName}/${buildName}
          '') buildTargets)}
        '') nodes')}
      '');

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
