package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"jirassic-capstone/internal/config"
	"jirassic-capstone/internal/ui"
)

func main() {
	// run the program and handle errors
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "jirassic:", err)
		os.Exit(1)
	}
}

func run() error {
	path, err := config.Path() // get the config path
	if err != nil {
		return err // if the config path is not found, return an error
	}
	cfg, err := config.Load(path) // load the config from the config path
	if err != nil {
		return err // if the config is not found, return an error
	}

	_, err = tea.NewProgram(ui.New(cfg, path), tea.WithAltScreen()).Run() // run the UI
	return err
}
