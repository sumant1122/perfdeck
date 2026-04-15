package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sumant1122/perfdeck/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.4.2"

func main() {
	if printVersion() {
		return
	}
	m := ui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printVersion() bool {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit")
	flag.Parse()
	if showVersion {
		fmt.Printf("perfdeck %s\n", version)
		return true
	}
	return false
}
