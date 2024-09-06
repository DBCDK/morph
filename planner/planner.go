package planner

import (
	"fmt"

	"github.com/DBCDK/morph/nix"
	"github.com/google/uuid"
)

type Step struct {
	Id          string
	Description string
	Action      string
	Parallel    bool
	OnFailure   string // retry, exit, ignore
	Steps       []Step
	Options     map[string]interface{}
	DependsOn   []string
}

type StepStatus struct {
	Id     string
	Status string
}

type StepData struct {
	Key   string
	Value string
}

func CreateStep(description string, action string, parallel bool, steps []Step, onFailure string, options map[string]interface{}, dependencies []string) Step {
	step := Step{
		Id:          uuid.New().String(),
		Description: description,
		Action:      action,
		Parallel:    parallel,
		Steps:       steps,
		OnFailure:   onFailure,
		Options:     options,
		DependsOn:   dependencies,
	}

	return step
}

func EmptyStep() Step {
	step := Step{
		Id:          uuid.New().String(),
		Description: "",
		Action:      "none",
		Parallel:    false,
		Steps:       make([]Step, 0),
		OnFailure:   "",
		Options:     make(map[string]interface{}, 0),
		DependsOn:   make([]string, 0),
	}

	return step
}

func AddSteps(plan Step, steps ...Step) Step {
	plan.Steps = append(plan.Steps, steps...)

	return plan
}

func EmptySteps() []Step {
	return make([]Step, 0)
}

func EmptyOptions() map[string]interface{} {
	return make(map[string]interface{}, 0)
}

func MakeDependencies(dependencies ...string) []string {
	deps := make([]string, 0)
	deps = append(deps, dependencies...)

	return deps
}

func CreateBuildPlan(hosts []nix.Host) Step {
	options := EmptyOptions()

	optionsHosts := make([]string, 0)
	for _, host := range hosts {
		optionsHosts = append(optionsHosts, host.Name)
	}

	options["hosts"] = optionsHosts

	return CreateStep("build hosts", "build", false, EmptySteps(), "exit", options, make([]string, 0))
}

func CreatePushPlan(buildId string, hosts []nix.Host) Step {
	options := EmptyOptions()

	pushParent := CreateStep("push to hosts", "none", true, EmptySteps(), "exit", options, MakeDependencies(buildId))

	for _, host := range hosts {
		pushToHostOptions := EmptyOptions()
		pushToHostOptions["to"] = host.Name
		pushParent = AddSteps(
			pushParent,
			CreateStep(fmt.Sprintf("push to %s", host.Name), "push", true, EmptySteps(), "exit", pushToHostOptions, MakeDependencies(buildId)),
		)
	}

	return pushParent
}
