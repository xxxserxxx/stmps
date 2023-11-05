package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type MenuWidget struct {
	Root *tview.Flex

	activeButton string
	buttons      map[string]*tview.Button

	// external references
	ui *Ui
}

var buttonOrder = []string{PageBrowser, PageQueue, PagePlaylists, PageLog}

func NewMenuWidget(ui *Ui) (m *MenuWidget) {
	m = &MenuWidget{
		activeButton: buttonOrder[0],
		buttons:      make(map[string]*tview.Button),

		ui: ui,
	}

	m.Root = tview.NewFlex().SetDirection(tview.FlexColumn)
	m.createButtons()
	m.updateButtons()

	return
}

func (m *MenuWidget) createButtons() {
	for i, page := range buttonOrder {
		button := tview.NewButton(page)
		button.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite))
		button.SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack))

		// create copy for our function
		buttonPage := page
		button.SetSelectedFunc(func() {
			m.ui.ShowPage(buttonPage)
		})

		m.buttons[page] = button
		// add button
		m.Root.AddItem(button, 15, 0, false)

		// add spacer
		if i < len(buttonOrder)-1 {
			m.Root.AddItem(nil, 1, 0, false)
		}
	}
}

func (m *MenuWidget) updateButtons() {
	for i, page := range buttonOrder {
		var text string
		if page == m.activeButton {
			text = fmt.Sprintf("%d: [::b]%s[::-]", i+1, page)
		} else {
			text = fmt.Sprintf("%d: %s", i+1, page)
		}

		m.buttons[page].SetLabel(text)
	}
}

func (m *MenuWidget) SetActivePage(name string) {
	if _, ok := m.buttons[name]; !ok {
		return // invalid button name
	}

	m.activeButton = name
	m.updateButtons()
}
