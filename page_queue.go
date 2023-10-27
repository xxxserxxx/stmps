// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/mpvplayer"
)

const queueDataColumns = 4
const starIcon = "♥"

type queueData struct {
	tview.TableContentReadOnly

	playerQueue mpvplayer.PlayerQueue
	starIdList  *map[string]struct{}
}

func (ui *Ui) createQueuePage() *tview.Flex {
	ui.queueList = tview.NewTable().
		SetSelectable(true, false) // rows selectable
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
	/*
		currentIndex := ui.queueList.GetCurrentItem()
		if currentIndex == -1 {
			return
		}

		// remove the item from the queue
		ui.player.DeleteQueueItem(currentIndex)
	*/
	ui.updateQueue()
}

func (ui *Ui) handleToggleStar() {
	/*currentIndex := ui.queueList.GetCurrentItem()
	if currentIndex < 0 {
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
	if ui.currentDirectory != nil {
		ui.handleEntitySelected(ui.currentDirectory.Id)
	}*/
}

func (ui *Ui) updateQueue() {
	ui.queueData.playerQueue = ui.player.GetQueueCopy()
	ui.queueData.starIdList = &ui.starIdList
	ui.queueList.SetContent(&ui.queueData)
}

func (q *queueData) GetCell(row, column int) *tview.TableCell {
	if row >= len(q.playerQueue) || column >= queueDataColumns {
		return nil
	}
	song := q.playerQueue[row]

	switch column {
	case 0: // star
		text := ""
		if _, starred := (*q.starIdList)[song.Id]; starred {
			text = starIcon
		}
		return &tview.TableCell{
			Text:          text,
			Color:         tcell.ColorRed,
			Expansion:     0,
			MaxWidth:      1,
			NotSelectable: true,
			Transparent:   true,
		}
	case 1: // title
		return &tview.TableCell{
			Text:        tview.Escape(song.Title),
			Expansion:   1,
			Transparent: true,
		}
	case 2: // artist
		return &tview.TableCell{
			Text:        tview.Escape(song.Artist),
			Expansion:   1,
			Transparent: true,
		}
	case 3: // duration
		min, sec := iSecondsToMinAndSec(song.Duration)
		text := fmt.Sprintf("%3d:%02d", min, sec)
		return &tview.TableCell{
			Text:        text,
			Align:       tview.AlignRight,
			Expansion:   0,
			MaxWidth:    6,
			Transparent: true,
		}
	}

	return nil
}

// Return the total number of rows in the table.
func (q *queueData) GetRowCount() int {
	return len(q.playerQueue)
}

// Return the total number of columns in the table.
func (q *queueData) GetColumnCount() int {
	return queueDataColumns
}
