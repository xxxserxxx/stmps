package main

import "github.com/rivo/tview"

func (ui *Ui) createLogPage(titleFlex *tview.Flex) *tview.Flex {

	ui.logList = tview.NewList().ShowSecondaryText(false)
	logListFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleFlex, 1, 0, false).
		AddItem(ui.logList, 0, 1, true)

	return logListFlex
}
