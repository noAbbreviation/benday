package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

type panicMsgModel string

func (_ panicMsgModel) Init() tea.Cmd {
	return nil
}

func (m panicMsgModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

func (panicMsg panicMsgModel) View() string {
	return string(panicMsg) + "\n"
}
