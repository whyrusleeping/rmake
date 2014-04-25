package client

import (
	"fmt"
)

func printHelpAdd() {
	fmt.Println("rmake add: Adds files to be used in the build process.")
}

func printHelpBin() {
	fmt.Println("rmake bin: set the name of the output binary to return.")
}

func printHelpScr() {
	fmt.Println("rmake scr: set the build command to run on the server.")
}

func printHelpServer() {
	fmt.Println("rmake server: set the url and port of the build server.")
}

func printHelpClean() {
	fmt.Println("rmake clean: resets mod times on your files and starts a new session with the build server.")
}

func printHelpVar() {
	fmt.Println("rmake var: ex: 'rmake var CFLAGS \"-O2 -g\"'")
	fmt.Println("\tSet environment variables on the build server.")
}

func printHelpCompress() {
	fmt.Println("rmake compress: 'rmake compress best'")
	fmt.Println("\tSet compression level for communications with the server.")
}

func printHelpStatus() {
	fmt.Println("rmake status: 'rmake status'")
	fmt.Println("\tShows tracked, untracked, and changed files.")
}

func printHelp(which string) {
	switch which {
	case "add":
		printHelpAdd()
	case "bin":
		printHelpBin()
	case "server":
		printHelpServer()
	case "scr":
		printHelpScr()
	case "clean":
		printHelpClean()
	case "compress":
		printHelpCompress()
	case "var":
		printHelpVar()
	case "status":
		printHelpStatus()
	default:
		fmt.Println("Usage: rmake [command] [args...]")
		printHelpAdd()
		printHelpBin()
		printHelpScr()
		printHelpServer()
		printHelpClean()
		printHelpVar()
		printHelpCompress()
		printHelpStatus()
	}
}
