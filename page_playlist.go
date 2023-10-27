// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/logger"
	"github.com/wildeyedskies/stmp/subsonic"
)

type PlaylistPage struct {
	Root                *tview.Flex
	DeletePlaylistModal tview.Primitive

	playlistList     *tview.List
	newPlaylistInput *tview.InputField
	selectedPlaylist *tview.List

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createPlaylistPage() *PlaylistPage {
	playlistPage := PlaylistPage{
		ui:     ui,
		logger: ui.logger,
	}

	// left half: playlists
	playlistPage.playlistList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	playlistPage.playlistList.Box.
		SetTitle(" playlist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// add the playlists
	for _, playlist := range ui.playlists {
		playlistPage.playlistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
	}

	// right half: songs of selected playlist
	playlistPage.selectedPlaylist = tview.NewList().
		ShowSecondaryText(false)
	playlistPage.selectedPlaylist.Box.
		SetTitle(" songs ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// flex wrapper
	playlistColFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(playlistPage.playlistList, 0, 1, true).
		AddItem(playlistPage.selectedPlaylist, 0, 1, false)

	// root view
	playlistPage.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(playlistColFlex, 0, 1, true)

	// input handlers
	playlistPage.newPlaylistInput = tview.NewInputField().
		SetLabel("Playlist name:").
		SetFieldWidth(50)
	playlistPage.newPlaylistInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			playlistPage.newPlaylist(playlistPage.newPlaylistInput.GetText())
			playlistPage.Root.Clear()
			playlistPage.Root.AddItem(playlistColFlex, 0, 1, true)
			ui.app.SetFocus(playlistPage.playlistList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			playlistPage.Root.Clear()
			playlistPage.Root.AddItem(playlistColFlex, 0, 1, true)
			ui.app.SetFocus(playlistPage.playlistList)
			return nil
		}
		return event
	})

	playlistPage.playlistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(playlistPage.selectedPlaylist)
			return nil
		}
		if event.Rune() == 'a' {
			playlistPage.handleAddPlaylistToQueue()
			return nil
		}
		if event.Rune() == 'n' {
			playlistPage.Root.AddItem(playlistPage.newPlaylistInput, 0, 1, true)
			ui.app.SetFocus(playlistPage.newPlaylistInput)
		}
		if event.Rune() == 'd' {
			ui.pages.ShowPage("deletePlaylist")
		}
		return event
	})

	playlistPage.selectedPlaylist.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(playlistPage.playlistList)
			return nil
		}
		if event.Rune() == 'a' {
			playlistPage.handleAddPlaylistSongToQueue()
			return nil
		}
		return event
	})

	// delete playlist modal
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
			playlistPage.deletePlaylist(playlistPage.playlistList.GetCurrentItem())
			ui.app.SetFocus(playlistPage.playlistList)
			ui.pages.HidePage("deletePlaylist")
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ui.app.SetFocus(playlistPage.playlistList)
			ui.pages.HidePage("deletePlaylist")
			return nil
		}
		return event
	})

	playlistPage.DeletePlaylistModal = makeModal(deletePlaylistFlex, 20, 3)

	playlistPage.playlistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index < 0 || index >= len(ui.playlists) {
			return
		}
		playlistPage.handlePlaylistSelected(ui.playlists[index])
	})

	// open first playlist by default so we don't get stuck when there's only one playlist
	if len(ui.playlists) > 0 {
		playlistPage.handlePlaylistSelected(ui.playlists[0])
	}

	return &playlistPage
}

func (p *PlaylistPage) IsNewPlaylistInputFocused(focused tview.Primitive) bool {
	return focused == p.newPlaylistInput
}

func (p *PlaylistPage) GetCount() int {
	return p.playlistList.GetItemCount()
}

func (p *PlaylistPage) UpdatePlaylists() {
	response, err := p.ui.connection.GetPlaylists()
	if err != nil {
		p.logger.PrintError("GetPlaylists", err)
	}
	p.ui.playlists = response.Playlists.Playlists

	p.playlistList.Clear()
	p.ui.addToPlaylistList.Clear()

	for _, playlist := range p.ui.playlists {
		p.playlistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
		p.ui.addToPlaylistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
	}
}

func (p *PlaylistPage) handleAddPlaylistSongToQueue() {
	playlistIndex := p.playlistList.GetCurrentItem()
	entityIndex := p.selectedPlaylist.GetCurrentItem()
	if playlistIndex < 0 || playlistIndex >= p.playlistList.GetItemCount() {
		return
	}
	if entityIndex < 0 || entityIndex >= p.selectedPlaylist.GetItemCount() {
		return
	}
	if playlistIndex >= len(p.ui.playlists) || entityIndex >= len(p.ui.playlists[playlistIndex].Entries) {
		return
	}

	// select next entry
	if entityIndex+1 < p.selectedPlaylist.GetItemCount() {
		p.selectedPlaylist.SetCurrentItem(entityIndex + 1)
	}

	entity := p.ui.playlists[playlistIndex].Entries[entityIndex]
	p.ui.addSongToQueue(&entity)

	p.ui.queuePage.UpdateQueue()
}

func (p *PlaylistPage) handleAddPlaylistToQueue() {
	currentIndex := p.playlistList.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= p.playlistList.GetItemCount() || currentIndex >= len(p.ui.playlists) {
		return
	}

	// focus next entry
	if currentIndex+1 < p.playlistList.GetItemCount() {
		p.playlistList.SetCurrentItem(currentIndex + 1)
	}

	playlist := p.ui.playlists[currentIndex]
	for _, entity := range playlist.Entries {
		p.ui.addSongToQueue(&entity)
	}

	p.ui.queuePage.UpdateQueue()
}

func (p *PlaylistPage) handlePlaylistSelected(playlist subsonic.SubsonicPlaylist) {
	p.selectedPlaylist.Clear()

	for _, entity := range playlist.Entries {
		handler := makeSongHandler(&entity, p.ui, entity.Artist)
		title := entity.GetSongTitle()
		p.selectedPlaylist.AddItem(tview.Escape(title), "", 0, handler)
	}
}

func (p *PlaylistPage) newPlaylist(name string) {
	response, err := p.ui.connection.CreatePlaylist(name)
	if err != nil {
		p.logger.Printf("newPlaylist: CreatePlaylist %s -- %s", name, err.Error())
		return
	}

	p.ui.playlists = append(p.ui.playlists, response.Playlist)

	p.playlistList.AddItem(tview.Escape(response.Playlist.Name), "", 0, nil)
	p.ui.addToPlaylistList.AddItem(tview.Escape(response.Playlist.Name), "", 0, nil)
}

func (p *PlaylistPage) deletePlaylist(index int) {
	if index < 0 || index >= len(p.ui.playlists) {
		return
	}

	playlist := p.ui.playlists[index]

	if index == 0 {
		p.playlistList.SetCurrentItem(1)
	}

	// Removes item with specified index
	p.ui.playlists = append(p.ui.playlists[:index], p.ui.playlists[index+1:]...)

	p.playlistList.RemoveItem(index)
	p.ui.addToPlaylistList.RemoveItem(index)
	if err := p.ui.connection.DeletePlaylist(string(playlist.Id)); err != nil {
		p.logger.PrintError("deletePlaylist", err)
	}
}
