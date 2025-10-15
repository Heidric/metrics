package main

import "os"

func run() {
	os.Exit(2) // want "direct call to os.Exit in package main is forbidden"
}

func main() {
	run()
}
