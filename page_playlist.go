package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *Ui) createPlaylistPage(titleFlex *tview.Flex) (*tview.Flex, tview.Primitive) {
	ui.playlistList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)

	//add the playlists
	for _, playlist := range ui.playlists {
		ui.playlistList.AddItem(playlist.Name, "", 0, nil)
	}

	ui.playlistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		ui.handlePlaylistSelected(ui.playlists[index])
	})

	playlistColFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.playlistList, 0, 1, true).
		AddItem(ui.selectedPlaylist, 0, 1, false)

	playlistFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleFlex, 1, 0, false).
		AddItem(playlistColFlex, 0, 1, true)

	ui.newPlaylistInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			ui.newPlaylist(ui.newPlaylistInput.GetText())
			playlistFlex.Clear()
			playlistFlex.AddItem(titleFlex, 1, 0, false)
			playlistFlex.AddItem(playlistColFlex, 0, 1, true)
			ui.app.SetFocus(ui.playlistList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			playlistFlex.Clear()
			playlistFlex.AddItem(titleFlex, 1, 0, false)
			playlistFlex.AddItem(playlistColFlex, 0, 1, true)
			ui.app.SetFocus(ui.playlistList)
			return nil
		}
		return event
	})

	ui.playlistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(ui.selectedPlaylist)
			return nil
		}
		if event.Rune() == 'a' {
			ui.handleAddPlaylistToQueue()
			return nil
		}
		if event.Rune() == 'n' {
			playlistFlex.AddItem(ui.newPlaylistInput, 0, 1, true)
			ui.app.SetFocus(ui.newPlaylistInput)
		}
		if event.Rune() == 'd' {
			ui.pages.ShowPage("deletePlaylist")
		}
		return event
	})

	ui.selectedPlaylist.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(ui.playlistList)
			return nil
		}
		if event.Rune() == 'a' {
			ui.handleAddPlaylistSongToQueue()
			return nil
		}
		return event
	})

	deletePlaylistList := tview.NewList().
		ShowSecondaryText(false)

	deletePlaylistList.AddItem("Confirm", "", 0, nil)

	deletePlaylistList.SetBorder(true).
		SetTitle("Confirm deletion")

	deletePlaylistFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(deletePlaylistList, 0, 1, true)

	deletePlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			ui.deletePlaylist(ui.playlistList.GetCurrentItem())
			ui.app.SetFocus(ui.playlistList)
			ui.pages.HidePage("deletePlaylist")
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ui.app.SetFocus(ui.playlistList)
			ui.pages.HidePage("deletePlaylist")
			return nil
		}
		return event
	})

	deletePlaylistModal := makeModal(deletePlaylistFlex, 20, 3)

	return playlistFlex, deletePlaylistModal
}
