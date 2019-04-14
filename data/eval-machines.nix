# Completely stripped down version of nixops' evaluator
{ networkExpr }:

let
  network = import networkExpr;
  netPkgs = network.network.pkgs;
  lib     = netPkgs.lib;
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

        customPath = lib.attrByPath [ "deployment" "nixPath" ] [] (network.${machineName} { config = {}; pkgs = {}; });

        # add path of network.pkgs if customPath is empty
        netPath = if customPath == [] then [ { prefix = "nixpkgs"; path = netPkgs.path; } ] else [];

        __nixPath = customPath ++ netPath ++ builtins.nixPath;

        # must stay before __nixPath so we resolve <nixpkgs> correctly
        importTarget = lib.attrByPath [ "deployment" "importPath" ] <nixpkgs/nixos/lib/eval-config.nix> (network.${machineName} { config = {}; pkgs = {}; });

        importFn =
          if customPath == [] then
            import
          else
            let
              overrides = {
                inherit __nixPath;
                import = fn: scopedImport overrides fn;
                scopedImport = attrs: fn: scopedImport (overrides // attrs) fn;
                builtins = builtins // overrides;
              };
            in
              scopedImport overrides;
      in
      { name = machineName;
        value = importFn importTarget {
          modules =
            modules ++
            [ { key = "deploy-stuff";
                imports = [ ./options.nix ];
                # Provide a default hostname and deployment target equal
                # to the attribute name of the machine in the model.
                networking.hostName = mkOverride 900 machineName;
                deployment.targetHost = mkOverride 900 machineName;
              }
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
          #nixosRelease = v.config.system.nixos.release or (removeSuffix v.config.system.nixos.version.suffix v.config.system.nixos.version);
        }
      );

    machineList = (map (key: getAttr key machines) (attrNames machines));
    network = network'.network or {};
  };

  # Phase 2: build complete machine configurations.
  machines = { names, buildTargets ? null }:
    let nodes' = filterAttrs (n: v: elem n names) nodes; in
    netPkgs.runCommand "morph"
      { preferLocalBuild = true; }
      (if buildTargets == null
      then ''
        mkdir -p $out
        ${toString (mapAttrsToList (nodeName: nodeDef: ''
          ln -s ${nodeDef.config.system.build.toplevel} $out/${nodeName}
          ln -s ${nodeDef.config.system.build.toplevel.drvPath} $out/${nodeName}.drv
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
