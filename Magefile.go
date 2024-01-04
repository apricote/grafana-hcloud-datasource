//go:build mage
// +build mage

package main

import (
	// mage:import
	build "github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Default configures the default target.
var Default = build.BuildAll

func Dev() {
	mg.Deps(build.Watch, DevFrontend, DevDockerCompose)
}

func DevFrontend() {
	sh.RunV("npm", "run", "dev")
}

func DevDockerCompose() {
	sh.RunV("docker", "compose", "up")
}
