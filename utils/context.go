package utils

import (
	"context"
	"time"
)

// Create a context with a timeout, but only if the timeout is longer than 0
func ContextWithConditionalTimeout(parent context.Context, timeout int) (context.Context, context.CancelFunc) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if timeout <= 0 {
		ctx, cancel = context.WithCancel(context.TODO())
	} else {
		ctx, cancel = context.WithTimeout(context.TODO(), time.Duration(timeout)*time.Second)
	}

	return ctx, cancel
}
