package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	BRAILLE_HEIGHT = 4
	BRAILLE_WIDTH  = 2
)

func main() {
	var model tea.Model

	switch {
	case hasStdinPipe():
		pixels, err := importPixelData(os.Stdin)
		if err != nil {
			fmt.Printf("Error: Cannot import from piped input: %v", err)
			os.Exit(1)
		}

		model = importCanvasModelFromArgs(pixels)

	case len(os.Args) >= 2:
		fileName := os.Args[1]
		model = previewArtModelFromArgs(fileName)

	default:
		model = newBendayStartModel()
	}

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func hasStdinPipe() bool {
	fileStat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return fileStat.Mode()&os.ModeNamedPipe != 0
}
