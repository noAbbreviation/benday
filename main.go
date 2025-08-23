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
	//  TODO: File explorer to select a file
	//  TODO: Piping to standard input to import braille ASCII to images
	model := tea.Model(newCreateCanvasModel())

	if len(os.Args) >= 2 {
		fileName := os.Args[1]
		model = newPreviewArtModel(fileName)
	}

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
