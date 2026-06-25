package main

import "github.com/liciomatos/pgloop/cmd"

// version is overwritten at build time via: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
