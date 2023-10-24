package main

import (
	"fmt"
	"math"
	"time"

	"github.com/rivo/tview"
	"github.com/wildeyedskies/go-mpv/mpv"
	"github.com/wildeyedskies/stmp/logger"
	"github.com/wildeyedskies/stmp/subsonic"
)

// struct contains all the updatable elements of the Ui
type Ui struct {
	app               *tview.Application
	pages             *tview.Pages
	entityList        *tview.List
	queueList         *tview.List
	playlistList      *tview.List
	addToPlaylistList *tview.List
	selectedPlaylist  *tview.List
	newPlaylistInput  *tview.InputField
	startStopStatus   *tview.TextView
	currentPage       *tview.TextView
	playerStatus      *tview.TextView
	logList           *tview.List
	searchField       *tview.InputField
	artistList        *tview.List

	currentDirectory *subsonic.SubsonicDirectory
	artistIdList     []string
	starIdList       map[string]struct{}
	playlists        []subsonic.SubsonicPlaylist

	connection *subsonic.SubsonicConnection
	player     *Player
	logger     *logger.Logger

	scrobbleTimer *time.Timer
}

func InitGui(indexes *[]subsonic.SubsonicIndex,
	playlists *[]subsonic.SubsonicPlaylist,
	connection *subsonic.SubsonicConnection,
	player *Player,
	logger *logger.Logger) *Ui {

	app := tview.NewApplication()
	pages := tview.NewPages()
	// player queue
	queueList := tview.NewList().ShowSecondaryText(false)
	// list of playlists
	playlistList := tview.NewList().ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	// same as 'playlistList' except for the addToPlaylistModal
	// - we need a specific version of this because we need different keybinds
	addToPlaylistList := tview.NewList().ShowSecondaryText(false)
	// songs in the selected playlist
	selectedPlaylist := tview.NewList().ShowSecondaryText(false)
	// status text at the top
	startStopStatus := tview.NewTextView().SetText("[::b]stmp: [red]stopped").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)
	currentPage := tview.NewTextView().SetText("Browser").
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	playerStatus := tview.NewTextView().SetText("[::b][100%][0:00/0:00]").
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)
	newPlaylistInput := tview.NewInputField().
		SetLabel("Playlist name:").
		SetFieldWidth(50)
	logs := tview.NewList().ShowSecondaryText(false)
	var currentDirectory *subsonic.SubsonicDirectory
	var artistIdList []string
	// Stores the song IDs
	var starIdList = map[string]struct{}{}

	// create reused timer to scrobble after delay
	scrobbleTimer := time.NewTimer(0)
	if !scrobbleTimer.Stop() {
		<-scrobbleTimer.C
	}

	ui := &Ui{
		app:               app,
		pages:             pages,
		queueList:         queueList,
		playlistList:      playlistList,
		addToPlaylistList: addToPlaylistList,
		selectedPlaylist:  selectedPlaylist,
		newPlaylistInput:  newPlaylistInput,
		startStopStatus:   startStopStatus,
		currentPage:       currentPage,
		playerStatus:      playerStatus,
		logList:           logs,

		currentDirectory: currentDirectory,
		artistIdList:     artistIdList,
		starIdList:       starIdList,
		playlists:        *playlists,

		connection: connection,
		player:     player,
		logger:     logger,

		scrobbleTimer: scrobbleTimer,
	}

	ui.addStarredToList()

	go func() {
		for {
			select {
			case msg := <-ui.logger.Prints:
				ui.app.QueueUpdate(func() {
					ui.logList.AddItem(msg, "", 0, nil)
					// Make sure the log list doesn't grow infinitely
					for ui.logList.GetItemCount() > 200 {
						ui.logList.RemoveItem(0)
					}
				})

			case <-scrobbleTimer.C:
				// scrobble submission delay elapsed
				paused, err := ui.player.IsPaused()
				ui.logger.Printf("scrobbler event: paused %v, err %v, qlen %d", paused, err, len(ui.player.Queue))
				isPlaying := err == nil && !paused
				if len(ui.player.Queue) > 0 && isPlaying {
					// it's still playing, submit it
					currentSong := ui.player.Queue[0]
					ui.connection.ScrobbleSubmission(currentSong.Id, true)
				}
			}
		}
	}()

	// create components shared by pages

	//title row flex
	titleFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.startStopStatus, 0, 1, false).
		AddItem(ui.currentPage, 0, 1, false).
		AddItem(ui.playerStatus, 0, 1, false)

	browserFlex, addToPlaylistModal := ui.createBrowserPage(titleFlex, indexes)
	queueFlex := ui.createQueuePage(titleFlex)
	playlistFlex, deletePlaylistModal := ui.createPlaylistPage(titleFlex)
	logListFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleFlex, 1, 0, false).
		AddItem(ui.logList, 0, 1, true)

	ui.pages.AddPage("browser", browserFlex, true, true).
		AddPage("queue", queueFlex, true, false).
		AddPage("playlists", playlistFlex, true, false).
		AddPage("addToPlaylist", addToPlaylistModal, true, false).
		AddPage("deletePlaylist", deletePlaylistModal, true, false).
		AddPage("log", logListFlex, true, false)

	// add page input handler
	ui.pages.SetInputCapture(ui.handlePageInput)

	// run mpv event handler
	go ui.handleMpvEvents()

	ui.app.SetRoot(ui.pages, true).
		SetFocus(ui.pages).
		EnableMouse(true)

	// run main loop
	if err := ui.app.Run(); err != nil {
		panic(err)
	}

	return ui
}

func (ui *Ui) handleMpvEvents() {
	ui.player.Instance.ObserveProperty(0, "time-pos", mpv.FORMAT_DOUBLE)
	ui.player.Instance.ObserveProperty(0, "duration", mpv.FORMAT_DOUBLE)
	ui.player.Instance.ObserveProperty(0, "volume", mpv.FORMAT_INT64)
	for evt := range ui.player.EventChannel {
		if evt == nil {
			// quit signal
			break
		} else if evt.Event_Id == mpv.EVENT_END_FILE && !ui.player.ReplaceInProgress {
			// we don't want to update anything if we're in the process of replacing the current track
			ui.startStopStatus.SetText("[::b]stmp: [red]stopped")

			// TODO it's gross that this is here, need better event handling
			if len(ui.player.Queue) > 0 {
				ui.player.Queue = ui.player.Queue[1:]
			}
			updateQueueList(ui.player, ui.queueList, ui.starIdList)
			err := ui.player.PlayNextTrack()
			if err != nil {
				ui.logger.Printf("handleMpvEvents: PlayNextTrack -- %s", err.Error())
			}
		} else if evt.Event_Id == mpv.EVENT_START_FILE {
			ui.player.ReplaceInProgress = false
			updateQueueList(ui.player, ui.queueList, ui.starIdList)

			if len(ui.player.Queue) > 0 {
				currentSong := ui.player.Queue[0]
				ui.startStopStatus.SetText("[::b]stmp: [green]playing " + currentSong.Title)

				if ui.connection.Scrobble {
					// scrobble "now playing" event
					ui.connection.ScrobbleSubmission(currentSong.Id, false)

					// scrobble "submission" after song has been playing a bit
					// see: https://www.last.fm/api/scrobbling
					// A track should only be scrobbled when the following conditions have been met:
					// The track must be longer than 30 seconds. And the track has been played for
					// at least half its duration, or for 4 minutes (whichever occurs earlier.)
					if currentSong.Duration > 30 {
						scrobbleDelay := currentSong.Duration / 2
						if scrobbleDelay > 240 {
							scrobbleDelay = 240
						}
						scrobbleDuration := time.Duration(scrobbleDelay) * time.Second

						ui.scrobbleTimer.Reset(scrobbleDuration)
						ui.logger.Printf("scrobbler: timer started, %v", scrobbleDuration)
					} else {
						ui.logger.Printf("scrobbler: track too short")
					}
				}
			}
		} else if evt.Event_Id == mpv.EVENT_IDLE || evt.Event_Id == mpv.EVENT_NONE {
			continue
		}

		position, err := ui.player.Instance.GetProperty("time-pos", mpv.FORMAT_DOUBLE)
		if err != nil {
			ui.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "time-pos", err.Error())
		}
		// TODO only update these as needed
		duration, err := ui.player.Instance.GetProperty("duration", mpv.FORMAT_DOUBLE)
		if err != nil {
			ui.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "duration", err.Error())
		}
		volume, err := ui.player.Instance.GetProperty("volume", mpv.FORMAT_INT64)
		if err != nil {
			ui.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "volume", err.Error())
		}

		if position == nil {
			position = 0.0
		}

		if duration == nil {
			duration = 0.0
		}

		if volume == nil {
			volume = 0
		}

		ui.playerStatus.SetText(formatPlayerStatus(volume.(int64), position.(float64), duration.(float64)))
		ui.app.Draw()
	}
}

func formatPlayerStatus(volume int64, position float64, duration float64) string {
	if position < 0 {
		position = 0.0
	}

	if duration < 0 {
		duration = 0.0
	}

	positionMin, positionSec := secondsToMinAndSec(position)
	durationMin, durationSec := secondsToMinAndSec(duration)

	return fmt.Sprintf("[::b][%d%%][%02d:%02d/%02d:%02d]", volume,
		positionMin, positionSec, durationMin, durationSec)
}

func secondsToMinAndSec(seconds float64) (int, int) {
	minutes := math.Floor(seconds / 60)
	remainingSeconds := int(seconds) % 60
	return int(minutes), remainingSeconds
}

func iSecondsToMinAndSec(seconds int) (int, int) {
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	return minutes, remainingSeconds
}

// if the first argument isn't empty, return it, otherwise return the second
func stringOr(firstChoice string, secondChoice string) string {
	if firstChoice != "" {
		return firstChoice
	}
	return secondChoice
}
