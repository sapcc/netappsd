package main

import (
	"fmt"
	"github.com/heptiolabs/healthcheck"
)

func ValueCheck(v *bool) healthcheck.Check {
	return func() error {
		if !*v {
			return fmt.Errorf("netbox query returns no data")
		}
		return nil
	}
}
