package cruft

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DBCDK/morph/common"
	"github.com/DBCDK/morph/filter"
	"github.com/DBCDK/morph/healthchecks"
	"github.com/DBCDK/morph/nix"
	"github.com/DBCDK/morph/ssh"
	"github.com/DBCDK/morph/utils"
)

func ExecBuild(opts *common.MorphOptions, hosts []nix.Host) (string, error) {
	resultPath, err := buildHosts(opts, hosts)
	if err != nil {
		return "", err
	}
	return resultPath, nil
}

func ExecDeploy(opts *common.MorphOptions, hosts []nix.Host) (string, error) {
	sshContext := ssh.CreateSSHContext(opts)

	doPush := false
	doUploadSecrets := false
	doActivate := false

	if !*opts.DryRun {
		switch opts.DeploySwitchAction {
		case "dry-activate":
			doPush = true
			doActivate = true
		case "test":
			fallthrough
		case "switch":
			fallthrough
		case "boot":
			doPush = true
			doUploadSecrets = opts.DeployUploadSecrets
			doActivate = true
		}
	}

	resultPath, err := buildHosts(opts, hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Deployment steps are disabled for build-only host: %s\n", host.Name)
			continue
		}

		singleHostInList := []nix.Host{host}

		if doPush {
			err = pushPaths(sshContext, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}
		fmt.Fprintln(os.Stderr)

		if doUploadSecrets {
			phase := "pre-activation"
			err = ExecUploadSecrets(opts, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !opts.SkipPreDeployChecks {
			err := healthchecks.PerformPreDeployChecks(sshContext, &host, opts.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host pre-deploy check failed.")
				utils.Exit(1)
			}
		}

		if doActivate {
			err = activateConfiguration(opts, singleHostInList, resultPath)
			if err != nil {
				return "", err
			}
		}

		if opts.DeployReboot {
			err = host.Reboot(sshContext)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Reboot failed")
				return "", err
			}
		}

		if doUploadSecrets {
			phase := "post-activation"
			err = ExecUploadSecrets(opts, singleHostInList, &phase)
			if err != nil {
				return "", err
			}

			fmt.Fprintln(os.Stderr)
		}

		if !opts.SkipHealthChecks {
			err := healthchecks.PerformHealthChecks(sshContext, &host, opts.Timeout)
			if err != nil {
				fmt.Fprintln(os.Stderr)
				fmt.Fprintln(os.Stderr, "Not deploying to additional hosts, since a host health check failed.")
				utils.Exit(1)
			}
		}

		fmt.Fprintln(os.Stderr, "Done:", host.Name)
	}

	return resultPath, nil
}

func ExecEval(opts *common.MorphOptions) (string, error) {
	deploymentFile, err := os.Open(opts.Deployment)
	deploymentPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return "", err
	}

	path, err := nix.GetNixContext(opts).EvalHosts(deploymentPath, opts.AttrKey)

	return path, err
}

func ExecExecute(opts *common.MorphOptions, hosts []nix.Host) error {
	sshContext := ssh.CreateSSHContext(opts)

	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Exec is disabled for build-only host: %s\n", host.Name)
			continue
		}
		fmt.Fprintln(os.Stderr, "** "+host.Name)
		sshContext.CmdInteractive(&host, opts.Timeout, opts.ExecuteCommand...)
		fmt.Fprintln(os.Stderr)
	}

	return nil
}

func ExecHealthCheck(opts *common.MorphOptions, hosts []nix.Host) error {
	sshContext := ssh.CreateSSHContext(opts)

	var err error
	for _, host := range hosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Healthchecks are disabled for build-only host: %s\n", host.Name)
			continue
		}
		err = healthchecks.PerformHealthChecks(sshContext, &host, opts.Timeout)
	}

	if err != nil {
		err = errors.New("One or more errors occurred during host healthchecks")
	}

	return err
}

func ExecPush(opts *common.MorphOptions, hosts []nix.Host) (string, error) {
	sshContext := ssh.CreateSSHContext(opts)

	resultPath, err := ExecBuild(opts, hosts)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr)
	return resultPath, pushPaths(sshContext, hosts, resultPath)
}

func GetHosts(opts *common.MorphOptions) (hosts []nix.Host, err error) {
	deploymentFile, err := os.Open(opts.Deployment)
	if err != nil {
		return hosts, err
	}

	deploymentAbsPath, err := filepath.Abs(deploymentFile.Name())
	if err != nil {
		return hosts, err
	}

	nixContext := nix.GetNixContext(opts)
	deployment, err := nixContext.GetMachines(deploymentAbsPath)
	if err != nil {
		return hosts, err
	}

	matchingHosts, err := filter.MatchHosts(deployment.Hosts, opts.SelectGlob)
	if err != nil {
		return hosts, err
	}

	var selectedTags []string
	if opts.SelectTags != "" {
		selectedTags = strings.Split(opts.SelectTags, ",")
	}

	matchingHosts2 := filter.FilterHostsTags(matchingHosts, selectedTags)

	ordering := deployment.Meta.Ordering
	if opts.OrderingTags != "" {
		ordering = nix.HostOrdering{Tags: strings.Split(opts.OrderingTags, ",")}
	}

	sortedHosts := filter.SortHosts(matchingHosts2, ordering)

	filteredHosts := filter.FilterHosts(sortedHosts, opts.SelectSkip, opts.SelectEvery, opts.SelectLimit)

	fmt.Fprintf(os.Stderr, "Selected %v/%v hosts (name filter:-%v, limits:-%v):\n", len(filteredHosts), len(deployment.Hosts), len(deployment.Hosts)-len(matchingHosts), len(matchingHosts)-len(filteredHosts))
	for index, host := range filteredHosts {
		fmt.Fprintf(os.Stderr, "\t%3d: %s (secrets: %d, health checks: %d, tags: %s)\n", index, host.Name, len(host.Secrets), len(host.HealthChecks.Cmd)+len(host.HealthChecks.Http), strings.Join(host.GetTags(), ","))
	}
	fmt.Fprintln(os.Stderr)

	return filteredHosts, nil
}

func activateConfiguration(opts *common.MorphOptions, filteredHosts []nix.Host, resultPath string) error {
	sshContext := ssh.CreateSSHContext(opts)

	fmt.Fprintln(os.Stderr, "Executing '"+opts.DeploySwitchAction+"' on matched hosts:")
	fmt.Fprintln(os.Stderr)
	for _, host := range filteredHosts {

		fmt.Fprintln(os.Stderr, "** "+host.Name)

		configuration, err := nix.GetNixSystemPath(host, resultPath)
		if err != nil {
			return err
		}

		err = sshContext.ActivateConfiguration(&host, configuration, opts.DeploySwitchAction)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr)
	}

	return nil
}

func buildHosts(opts *common.MorphOptions, hosts []nix.Host) (resultPath string, err error) {
	if len(hosts) == 0 {
		err = errors.New("No hosts selected")
		return
	}

	deploymentPath, err := filepath.Abs(opts.Deployment)
	if err != nil {
		return
	}

	nixBuildTargets := ""
	if opts.NixBuildTargetFile != "" {
		if path, err := filepath.Abs(opts.NixBuildTargetFile); err == nil {
			nixBuildTargets = fmt.Sprintf("import \"%s\"", path)
		}
	} else if opts.NixBuildTarget != "" {
		nixBuildTargets = fmt.Sprintf("{ \"out\" = %s; }", opts.NixBuildTarget)
	}

	nixContext := nix.GetNixContext(opts)
	resultPath, err = nixContext.BuildMachines(deploymentPath, hosts, nixBuildTargets)

	if err != nil {
		return
	}

	fmt.Fprintln(os.Stderr, "nix result path: ")
	fmt.Println(resultPath)
	return
}

func pushPaths(sshContext *ssh.SSHContext, filteredHosts []nix.Host, resultPath string) error {
	for _, host := range filteredHosts {
		if host.BuildOnly {
			fmt.Fprintf(os.Stderr, "Push is disabled for build-only host: %s\n", host.Name)
			continue
		}

		paths, err := nix.GetPathsToPush(host, resultPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Pushing paths to %v (%v@%v):\n", host.Name, host.TargetUser, host.TargetHost)
		for _, path := range paths {
			fmt.Fprintf(os.Stderr, "\t* %s\n", path)
		}
		err = nix.Push(sshContext, host, paths...)
		if err != nil {
			return err
		}
	}

	return nil
}
