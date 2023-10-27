// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/logger"
	"github.com/wildeyedskies/stmp/mpvplayer"
	"github.com/wildeyedskies/stmp/subsonic"
)

// struct contains all the updatable elements of the Ui
type Ui struct {
	app   *tview.Application
	pages *tview.Pages

	// top row
	startStopStatus *tview.TextView
	currentPage     *tview.TextView
	playerStatus    *tview.TextView

	// browser page
	browserPage *BrowserPage

	// queue page
	queuePage *QueuePage

	// playlist page
	playlistPage *PlaylistPage

	// log page
	logPage *LogPage

	// modals
	addToPlaylistList *tview.List
	messageBox        *tview.Modal

	starIdList map[string]struct{}

	eventLoop *eventLoop
	mpvEvents chan mpvplayer.UiEvent

	playlists  []subsonic.SubsonicPlaylist
	connection *subsonic.SubsonicConnection
	player     *mpvplayer.Player
	logger     *logger.Logger
}

func InitGui(indexes *[]subsonic.SubsonicIndex,
	playlists *[]subsonic.SubsonicPlaylist,
	connection *subsonic.SubsonicConnection,
	player *mpvplayer.Player,
	logger *logger.Logger) (ui *Ui) {
	ui = &Ui{
		starIdList: map[string]struct{}{},

		eventLoop: nil, // initialized by initEventLoops()
		mpvEvents: make(chan mpvplayer.UiEvent, 5),

		playlists:  *playlists,
		connection: connection,
		player:     player,
		logger:     logger,
	}

	ui.initEventLoops()

	ui.app = tview.NewApplication()
	ui.pages = tview.NewPages()

	// status text at the top
	ui.startStopStatus = tview.NewTextView().SetText("[::b]stmp").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)
	ui.playerStatus = tview.NewTextView().SetText("[100%][::b][00:00/00:00]").
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)

	ui.currentPage = tview.NewTextView().SetText("Browser").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// same as 'playlistList' except for the addToPlaylistModal
	// - we need a specific version of this because we need different keybinds
	ui.addToPlaylistList = tview.NewList().ShowSecondaryText(false)

	// message box for small notes
	ui.messageBox = tview.NewModal().
		SetText("hi there").
		SetBackgroundColor(tcell.ColorBlack)
	ui.messageBox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		ui.pages.HidePage("messageBox")
		return event
	})

	// top row
	top1Flex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.startStopStatus, 0, 1, false).
		AddItem(ui.playerStatus, 20, 0, false)

	// 2nd row
	top2Flex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.currentPage, 0, 1, false)

	// browser page
	ui.browserPage = ui.createBrowserPage(indexes)

	// queue page
	ui.queuePage = ui.createQueuePage()

	// playlist page
	ui.playlistPage = ui.createPlaylistPage()

	// log page
	ui.logPage = ui.createLogPage()

	ui.pages.AddPage("browser", ui.browserPage.Root, true, true).
		AddPage("queue", ui.queuePage.Root, true, false).
		AddPage("playlists", ui.playlistPage.Root, true, false).
		AddPage("addToPlaylist", ui.browserPage.AddToPlaylistModal, true, false).
		AddPage("deletePlaylist", ui.playlistPage.DeletePlaylistModal, true, false).
		AddPage("messageBox", ui.messageBox, true, false).
		AddPage("log", ui.logPage.Root, true, false)

	rootFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(top1Flex, 1, 0, false).
		AddItem(top2Flex, 1, 0, false).
		AddItem(ui.pages, 0, 1, true)

	// add main input handler
	rootFlex.SetInputCapture(ui.handlePageInput)

	ui.app.SetRoot(rootFlex, true).
		SetFocus(rootFlex).
		EnableMouse(true)

	return ui
}

func (ui *Ui) Run() error {
	// receive events from mpv wrapper
	ui.player.RegisterEventConsumer(ui)

	// run gui/background event handler
	ui.runEventLoops()

	// run mpv event handler
	go ui.player.EventLoop()

	// gui main loop (blocking)
	return ui.app.Run()
}

func (ui *Ui) showMessageBox(text string) {
	ui.pages.ShowPage("messageBox")
	ui.messageBox.SetText(text)
	ui.app.SetFocus(ui.messageBox)
}
