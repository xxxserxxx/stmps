package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *Ui) createQueuePage() *tview.Flex {
	ui.queueList = tview.NewList().
		ShowSecondaryText(false)
	ui.queueList.Box.
		SetTitle(" queue ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	queueFlex := tview.NewFlex().SetDirection(tview.FlexRow).
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

func (ui *Ui) handleDeleteFromQueue() {
	currentIndex := ui.queueList.GetCurrentItem()
	if currentIndex == -1 /*|| len(ui.player.Queue) < currentIndex*/ {
		return
	}

	// remove the item from the queue
	ui.player.DeleteQueueItem(currentIndex)

	updateQueueList(ui.player, ui.queueList, ui.starIdList)
}

func (ui *Ui) handleToggleStar() {
	currentIndex := ui.queueList.GetCurrentItem()
	if currentIndex < 0 /*|| len(ui.player.Queue) < currentIndex*/ {
		return
	}

	entity, err := ui.player.GetQueueItem(currentIndex)
	if err != nil {
		ui.logger.PrintError("handleToggleStar", err)
		return
	}

	// If the song is already in the star list, remove it
	_, remove := ui.starIdList[entity.Id]

	// resp, _ := ui.connection.ToggleStar(entity.Id, remove)
	if _, err = ui.connection.ToggleStar(entity.Id, ui.starIdList); err != nil {
		ui.showMessageBox("ToggleStar failed")
		return
	}

	if remove {
		delete(ui.starIdList, entity.Id)
	} else {
		ui.starIdList[entity.Id] = struct{}{}
	}

	var text = queueListTextFormat(entity, ui.starIdList)
	updateQueueListItem(ui.queueList, currentIndex, text)
	// Update the entity list to reflect any changes
	ui.logger.Printf("entity test %v", ui.currentDirectory)
	if ui.currentDirectory != nil {
		ui.handleEntitySelected(ui.currentDirectory.Id)
	}
}
