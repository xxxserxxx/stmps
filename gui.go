package main

import (
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/logger"
	"github.com/wildeyedskies/stmp/mpv"
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
	artistList  *tview.List
	entityList  *tview.List
	searchField *tview.InputField

	// queue page
	queueList *tview.List

	// playlist page
	playlistList *tview.List

	// log page
	logList *tview.List

	addToPlaylistList *tview.List
	selectedPlaylist  *tview.List
	newPlaylistInput  *tview.InputField

	currentDirectory *subsonic.SubsonicDirectory
	artistIdList     []string
	starIdList       map[string]struct{}

	eventLoop *eventLoop
	mpvEvents chan mpv.UiEvent

	playlists  []subsonic.SubsonicPlaylist
	connection *subsonic.SubsonicConnection
	player     *mpv.Player
	logger     *logger.Logger
}

func InitGui(indexes *[]subsonic.SubsonicIndex,
	playlists *[]subsonic.SubsonicPlaylist,
	connection *subsonic.SubsonicConnection,
	player *mpv.Player,
	logger *logger.Logger) (ui *Ui) {
	ui = &Ui{
		currentDirectory: &subsonic.SubsonicDirectory{},
		artistIdList:     []string{},
		starIdList:       map[string]struct{}{},

		eventLoop: nil, // initialized by initEventLoops()
		mpvEvents: make(chan mpv.UiEvent, 5),

		playlists:  *playlists,
		connection: connection,
		player:     player,
		logger:     logger,
	}

	ui.initEventLoops()

	ui.app = tview.NewApplication()
	ui.pages = tview.NewPages()

	// status text at the top
	ui.startStopStatus = tview.NewTextView().SetText("[::b]stmp: [red]stopped").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)
	ui.currentPage = tview.NewTextView().SetText("Browser").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	ui.playerStatus = tview.NewTextView().SetText("[::b][100%][0:00/0:00]").
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)

	// same as 'playlistList' except for the addToPlaylistModal
	// - we need a specific version of this because we need different keybinds
	ui.addToPlaylistList = tview.NewList().ShowSecondaryText(false)
	// songs in the selected playlist
	ui.selectedPlaylist = tview.NewList().ShowSecondaryText(false)
	ui.newPlaylistInput = tview.NewInputField().
		SetLabel("Playlist name:").
		SetFieldWidth(50)

	// top row
	titleFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.startStopStatus, 0, 1, false).
		AddItem(ui.currentPage, 0, 1, false).
		AddItem(ui.playerStatus, 0, 1, false)

	// browser page
	browserFlex, addToPlaylistModal := ui.createBrowserPage(titleFlex, indexes)

	// queue page
	queueFlex := ui.createQueuePage(titleFlex)

	// playlist page
	playlistFlex, deletePlaylistModal := ui.createPlaylistPage(titleFlex)

	// log page
	logListFlex := ui.createLogPage(titleFlex)

	ui.pages.AddPage("browser", browserFlex, true, true).
		AddPage("queue", queueFlex, true, false).
		AddPage("playlists", playlistFlex, true, false).
		AddPage("addToPlaylist", addToPlaylistModal, true, false).
		AddPage("deletePlaylist", deletePlaylistModal, true, false).
		AddPage("log", logListFlex, true, false)

	// add page input handler
	ui.pages.SetInputCapture(ui.handlePageInput)

	ui.app.SetRoot(ui.pages, true).
		SetFocus(ui.pages).
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
