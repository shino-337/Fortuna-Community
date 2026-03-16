package main

import (
	_ "github.com/sirupsen/logrus"
)

// This simple program exists only to produce a small Go binary with buildinfo
// for the GoBinaryParser integration test. It imports logrus so that at least
// one dependency module is present in buildinfo.Deps.

func main() {
	println("gobinary test with logrus")
}


