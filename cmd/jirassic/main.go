package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "jirassic:", err)
		os.Exit(1)
	}
}

func run() error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(ui.New(cfg, path), tea.WithAltScreen()).Run()
	return err
}
