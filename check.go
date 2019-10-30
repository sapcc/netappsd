package main

import (
	"fmt"
	"github.com/heptiolabs/healthcheck"
)

func ValueCheck(msg string, fn func() bool) healthcheck.Check {
	return func() error {
		if ck := fn(); !ck {
			return fmt.Errorf(msg)
		}
		return nil
	}
}
