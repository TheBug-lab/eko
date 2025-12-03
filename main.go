package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thebug/lab/eko/v3/pkg/ui"
)

func main() {
	imageMode := flag.Bool("i", false, "Enable image generation mode")
	flag.Parse()

	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic recovered: %v\n", r)
			os.Exit(1)
		}
	}()

	p := tea.NewProgram(ui.NewModel(*imageMode, flag.Args()), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
