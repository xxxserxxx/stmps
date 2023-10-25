package main

import "github.com/rivo/tview"

func (ui *Ui) createLogPage() *tview.Flex {

	ui.logList = tview.NewList().ShowSecondaryText(false)
	logListFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.logList, 0, 1, true)

	return logListFlex
}
