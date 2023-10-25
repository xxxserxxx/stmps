package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *Ui) createQueuePage(titleFlex *tview.Flex) *tview.Flex {
	ui.queueList = tview.NewList().ShowSecondaryText(false)

	queueFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleFlex, 1, 0, false).
		AddItem(ui.queueList, 0, 1, true)

	ui.queueList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDelete || event.Rune() == 'd' {
			ui.handleDeleteFromQueue()
			return nil
		} else if event.Rune() == 'y' {
			ui.handleToggleStar()
			return nil
		}

		return event
	})

	return queueFlex
}
