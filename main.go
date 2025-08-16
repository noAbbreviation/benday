package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type canvasModel struct {
	pixels   [][]bool
	fileName string
}

func newCanvas(fileName string) canvasModel {
	return canvasModel{
		pixels:   [][]bool{},
		fileName: fileName,
	}
}

func (m canvasModel) Init() tea.Cmd {
	return tea.SetWindowTitle("benday")
}

func (m canvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "e":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m canvasModel) View() string {
	render := []string{
		"Hello world!",
		renderBraille(m.pixels),
		"(Ctrl-C or q to quit)",
	}

	return strings.Join(render, "\n")
}

func renderBraille(_ [][]bool) string {
	result := ""
	for i, braille := range brailleLookup {
		if i%32 == 0 {
			result += "\n"
		}

		result += string(braille)
	}

	return result
}

func main() {
	//  TODO: File explorer to select a file
	model := tea.Model(newCreateCanvasModel())

	if len(os.Args) >= 2 {
		fileName := os.Args[1]
		model = newCanvas(fileName)
	}

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
