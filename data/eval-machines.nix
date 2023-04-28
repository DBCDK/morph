# Completely stripped down version of nixops' evaluator
{ networkExpr }:

let
  network        = import networkExpr;
  nw             = network.network;
  nwPkgs         = nw.pkgs or {};
  nwLib          = nw.lib or nwPkgs.lib or (import <nixpkgs/lib>);
  nwEvalConfig   = let
                     nwEvalConfig' =
                       nw.evalConfig or ((nwPkgs.path or <nixpkgs>) + "/nixos/lib/eval-config.nix");
                   in
                   if nwLib.isFunction nwEvalConfig' then nwEvalConfig' else import nwEvalConfig';
  nwSpecialArgs  = nw.specialArgs or {};
  nwRunCommand   = nw.runCommand or nwPkgs.runCommand or ((import <nixpkgs> {}).runCommand);
in

let
  modules = { machineName, machineModule ? network.${machineName}, nodes, check }: [
    # Get the configuration of this machine from each network
    # expression, attaching _file attributes so the NixOS module
    # system can give sensible error messages.
    { imports = [ machineModule ]; }

    ({ config, lib, options, ... }: {
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
      nixpkgs.pkgs = lib.mkIf (nwPkgs != {}) (lib.mkDefault nwPkgs);

      # Avoid the deprecated evalConfig arguments by
      # setting them here instead.
      _module = {
        args = {
          name = machineName;
          inherit nodes;
        };
        inherit check;
      };
    })
  ] ++ nwLib.optional (network ? _file) { inherit (network) _file; };

  networkMachines = removeAttrs network [ "network" "defaults" "resources" "require" "_file" ];

in rec {
  # Unchecked configuration of all machines.
  # Using unchecked config evaluation allows each machine to access other machines
  # configuration without recursing as full evaluation is prevented
  uncheckedNodes =
    builtins.mapAttrs (machineName: machineModule: nwEvalConfig {
      modules = modules {
        inherit machineName machineModule;
        check = false;
        nodes = uncheckedNodes;
      };
      specialArgs = nwSpecialArgs;
    }) networkMachines;

  # Compute the definitions of the machines.
  nodes =
    builtins.mapAttrs (machineName: machineModule: nwEvalConfig {
      modules = modules {
        inherit machineName;
        check = true;
        nodes = uncheckedNodes;
      };
      specialArgs = nwSpecialArgs;
    }) networkMachines;

  deploymentInfoModule = {
    deployment = {
      name = nwLib.deploymentName;
      arguments = nwLib.args;
      inherit (nwLib) uuid;
    };
  };

  # Phase 1: evaluate only the deployment attributes.
  info =
    let
      network' = network;
      nodes' = nodes;
    in rec {

    machines =
      builtins.mapAttrs (n: v': let v = nwLib.scrubOptionValue v'; in
        { inherit (v.config.deployment) targetHost targetPort targetUser secrets healthChecks buildOnly substituteOnDestination tags;
          name = n;
          nixosRelease = v.config.system.nixos.release or (nwLib.removeSuffix v.config.system.nixos.version.suffix v.config.system.nixos.version);
          nixConfig = builtins.mapAttrs
            (n: v: if builtins.isString v then v else throw "nix option '${n}' must have a string typed value")
            (network'.network.nixConfig or {});
        }
      ) nodes;

    machineList = (map (key: builtins.getAttr key machines) (builtins.attrNames machines));
    network = network'.network or {};
    deployment = {
      hosts = machineList;
      meta = {
        description = network.description or "";
        ordering = network.ordering or {};
      };
    };

    buildShell = network.buildShell.drvPath or null;
  };

  # Phase 2: build complete machine configurations.
  machines = { argsFile, buildTargets ? null }:
    let
      fileArgs = builtins.fromJSON (builtins.readFile argsFile);
      nodes' = nwLib.filterAttrs (n: v: builtins.elem n fileArgs.Names) nodes; in
    nwRunCommand "morph"
      { preferLocalBuild = true; }
      (if buildTargets == null
      then ''
        mkdir -p $out
        ${toString (nwLib.mapAttrsToList (nodeName: nodeDef: ''
          ln -s ${nodeDef.config.system.build.toplevel} $out/${nodeName}
        '') nodes')}
      ''
      else ''
        mkdir -p $out
        ${toString (nwLib.mapAttrsToList (nodeName: nodeDef: ''
          mkdir -p $out/${nodeName}
          ${toString (nwLib.mapAttrsToList (buildName: buildFn: ''
            ln -s ${buildFn nodeDef} $out/${nodeName}/${buildName}
          '') buildTargets)}
        '') nodes')}
      '');

}
