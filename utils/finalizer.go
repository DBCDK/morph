package utils

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

type finalizer struct {
	function FinalizerFunc
	executed bool
}
type FinalizerFunc func()

var finalizers []*finalizer

/*
	Finalizers run sequentially at morph shutdown - both at clean shutdown and on errors.
	Finalizers should be quick, simple and they need to ignore errors and don't panic.
	Each finalizer will only run once and will _never_ be re-invoked.
*/
func (f *finalizer) Run() {
	if !f.executed {
		f.executed = true
		f.function()
	}
}

func RunFinalizers() {
	for _, f := range finalizers {
		f.Run()
	}
}

func AddFinalizer(f FinalizerFunc) {
	finalizers = append(finalizers, &finalizer{
		function: f,
		executed: false,
	})
}

func SignalHandler() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Fprintf(os.Stderr, "Received signal: %s\n", sig.String())
		Exit(130) // reserved exit code for "Interrupted"
	}()
}

func Exit(exitCode int) {
	RunFinalizers()
	os.Exit(exitCode)
}
