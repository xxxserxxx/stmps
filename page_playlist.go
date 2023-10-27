// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/subsonic"
)

func (ui *Ui) createPlaylistPage() (*tview.Flex, tview.Primitive) {
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
		AddItem(playlistColFlex, 0, 1, true)

	ui.newPlaylistInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			ui.newPlaylist(ui.newPlaylistInput.GetText())
			playlistFlex.Clear()
			playlistFlex.AddItem(playlistColFlex, 0, 1, true)
			ui.app.SetFocus(ui.playlistList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			playlistFlex.Clear()
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

func (ui *Ui) handleAddPlaylistSongToQueue() {
	playlistIndex := ui.playlistList.GetCurrentItem()
	entityIndex := ui.selectedPlaylist.GetCurrentItem()

	if playlistIndex < 0 || entityIndex < 0 {
		return
	}

	if entityIndex+1 < ui.selectedPlaylist.GetItemCount() {
		ui.selectedPlaylist.SetCurrentItem(entityIndex + 1)
	}

	// TODO add some bounds checking here
	if playlistIndex == -1 || entityIndex == -1 {
		return
	}

	entity := ui.playlists[playlistIndex].Entries[entityIndex]
	ui.addSongToQueue(&entity)

	ui.updateQueue()
}

func (ui *Ui) handleAddPlaylistToQueue() {
	currentIndex := ui.playlistList.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= ui.playlistList.GetItemCount() {
		return
	}

	// focus next entry
	if currentIndex+1 < ui.playlistList.GetItemCount() {
		ui.playlistList.SetCurrentItem(currentIndex + 1)
	}

	playlist := ui.playlists[currentIndex]

	for _, entity := range playlist.Entries {
		ui.addSongToQueue(&entity)
	}

	ui.updateQueue()
}

func (ui *Ui) handlePlaylistSelected(playlist subsonic.SubsonicPlaylist) {
	ui.selectedPlaylist.Clear()

	for _, entity := range playlist.Entries {
		var title string
		var handler func()

		var id = entity.Id

		title = entity.GetSongTitle()
		handler = makeSongHandler(id, ui.connection.GetPlayUrl(&entity), title, entity.Artist, entity.Duration, ui)

		ui.selectedPlaylist.AddItem(title, "", 0, handler)
	}
}

func (ui *Ui) newPlaylist(name string) {
	response, err := ui.connection.CreatePlaylist(name)
	if err != nil {
		ui.logger.Printf("newPlaylist: CreatePlaylist %s -- %s", name, err.Error())
		return
	}

	ui.playlists = append(ui.playlists, response.Playlist)

	ui.playlistList.AddItem(response.Playlist.Name, "", 0, nil)
	ui.addToPlaylistList.AddItem(response.Playlist.Name, "", 0, nil)
}

func (ui *Ui) deletePlaylist(index int) {
	if index == -1 || len(ui.playlists) <= index {
		return
	}

	playlist := ui.playlists[index]

	if index == 0 {
		ui.playlistList.SetCurrentItem(1)
	}

	// Removes item with specified index
	ui.playlists = append(ui.playlists[:index], ui.playlists[index+1:]...)

	ui.playlistList.RemoveItem(index)
	ui.addToPlaylistList.RemoveItem(index)
	if err := ui.connection.DeletePlaylist(string(playlist.Id)); err != nil {
		ui.logger.PrintError("deletePlaylist", err)
	}
}
