package main

import (
	"fmt"
	"math"
	"time"

	"github.com/gdamore/tcell/v2"
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
				connection.Logger.Printf("scrobbler event: paused %v, err %v, qlen %d", paused, err, len(ui.player.Queue))
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

func (ui *Ui) createBrowserPage(titleFlex *tview.Flex, indexes *[]subsonic.SubsonicIndex) (*tview.Flex, tview.Primitive) {
	// artist list
	ui.artistList = tview.NewList().
		ShowSecondaryText(false)
	ui.artistList.Box.
		SetTitle(" Artist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	for _, index := range *indexes {
		for _, artist := range index.Artists {
			ui.artistList.AddItem(artist.Name, "", 0, nil)
			ui.artistIdList = append(ui.artistIdList, artist.Id)
		}
	}

	// album list
	ui.entityList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	ui.entityList.Box.
		SetTitle(" Album ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	ui.searchField = tview.NewInputField().
		SetLabel("Search:").
		SetChangedFunc(func(s string) {
			idxs := ui.artistList.FindItems(s, "", false, true)
			if len(idxs) == 0 {
				return
			}
			ui.artistList.SetCurrentItem(idxs[0])
		}).SetDoneFunc(func(key tcell.Key) {
		ui.app.SetFocus(ui.artistList)
	})

	artistFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.artistList, 0, 1, true).
		AddItem(ui.entityList, 0, 1, false)

	browserFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleFlex, 1, 0, false).
		AddItem(artistFlex, 0, 1, true).
		AddItem(ui.searchField, 1, 0, false)

	// going right from the artist list should focus the album/song list
	ui.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(ui.entityList)
			return nil
		}
		switch event.Rune() {
		case '/':
			ui.search()
			return nil
		case 'n':
			ui.searchNext()
			return nil
		case 'N':
			ui.searchPrev()
			return nil
		case 'r':
			goBackTo := ui.artistList.GetCurrentItem()
			// REFRESH artists
			indexResponse, err := ui.connection.GetIndexes()
			if err != nil {
				ui.connection.Logger.Printf("Error fetching indexes from server: %s\n", err)
				return event
			}
			ui.artistList.Clear()
			ui.connection.ClearCache()
			for _, index := range indexResponse.Indexes.Index {
				for _, artist := range index.Artists {
					ui.artistList.AddItem(artist.Name, "", 0, nil)
					ui.artistIdList = append(ui.artistIdList, artist.Id)
				}
			}
			// Try to put the user to about where they were
			if goBackTo < ui.artistList.GetItemCount() {
				ui.artistList.SetCurrentItem(goBackTo)
			}
		}
		return event
	})

	ui.artistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		ui.handleEntitySelected(ui.artistIdList[index])
	})

	for _, playlist := range ui.playlists {
		ui.addToPlaylistList.AddItem(playlist.Name, "", 0, nil)
	}
	ui.addToPlaylistList.SetBorder(true).
		SetTitle("Add to Playlist")

	addToPlaylistFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.addToPlaylistList, 0, 1, true)

	addToPlaylistModal := makeModal(addToPlaylistFlex, 60, 20)

	ui.addToPlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.pages.HidePage("addToPlaylist")
			ui.pages.SwitchToPage("browser")
			ui.app.SetFocus(ui.entityList)
		} else if event.Key() == tcell.KeyEnter {
			playlist := ui.playlists[ui.addToPlaylistList.GetCurrentItem()]
			ui.handleAddSongToPlaylist(&playlist)

			ui.pages.HidePage("addToPlaylist")
			ui.pages.SwitchToPage("browser")
			ui.app.SetFocus(ui.entityList)
		}
		return event
	})

	ui.entityList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(ui.artistList)
			return nil
		}
		if event.Rune() == 'a' {
			ui.handleAddEntityToQueue()
			return nil
		}
		if event.Rune() == 'y' {
			ui.handleToggleEntityStar()
			return nil
		}
		// only makes sense to add to a playlist if there are playlists
		if event.Rune() == 'A' && ui.playlistList.GetItemCount() > 0 {
			ui.pages.ShowPage("addToPlaylist")
			ui.app.SetFocus(ui.addToPlaylistList)
			return nil
		}
		// REFRESH only the artist
		if event.Rune() == 'r' {
			artistIdx := ui.artistList.GetCurrentItem()
			entity := ui.artistIdList[artistIdx]
			//ui.logger.Printf("refreshing artist idx %d, entity %s (%s)", artistIdx, entity, ui.connection.directoryCache[entity].Directory.Name)
			ui.connection.RemoveCacheEntry(entity)
			ui.handleEntitySelected(ui.artistIdList[artistIdx])
			return nil
		}
		return event
	})

	return browserFlex, addToPlaylistModal
}

func (ui *Ui) createQueuePage(titleFlex *tview.Flex) *tview.Flex {
	queueFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleFlex, 1, 0, false).
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

func (ui *Ui) createPlaylistPage(titleFlex *tview.Flex) (*tview.Flex, tview.Primitive) {
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

func queueListTextFormat(queueItem QueueItem, starredItems map[string]struct{}) string {
	min, sec := iSecondsToMinAndSec(queueItem.Duration)
	var star = ""
	_, hasStar := starredItems[queueItem.Id]
	if hasStar {
		star = " [red]♥"
	}
	return fmt.Sprintf("%s - %s - %02d:%02d %s", queueItem.Title, queueItem.Artist, min, sec, star)
}

// Just update the text of a specific row
func updateQueueListItem(queueList *tview.List, id int, text string) {
	queueList.SetItemText(id, text, "")
}

func updateQueueList(player *Player, queueList *tview.List, starredItems map[string]struct{}) {
	queueList.Clear()
	for _, queueItem := range player.Queue {
		queueList.AddItem(queueListTextFormat(queueItem, starredItems), "", 0, nil)
	}
}

func (ui *Ui) handleMpvEvents() {
	ui.player.Instance.ObserveProperty(0, "time-pos", mpv.FORMAT_DOUBLE)
	ui.player.Instance.ObserveProperty(0, "duration", mpv.FORMAT_DOUBLE)
	ui.player.Instance.ObserveProperty(0, "volume", mpv.FORMAT_INT64)
	for {
		e := <-ui.player.EventChannel
		if e == nil {
			break
			// we don't want to update anything if we're in the process of replacing the current track
		} else if e.Event_Id == mpv.EVENT_END_FILE && !ui.player.ReplaceInProgress {
			ui.startStopStatus.SetText("[::b]stmp: [red]stopped")
			// TODO it's gross that this is here, need better event handling
			if len(ui.player.Queue) > 0 {
				ui.player.Queue = ui.player.Queue[1:]
			}
			updateQueueList(ui.player, ui.queueList, ui.starIdList)
			err := ui.player.PlayNextTrack()
			if err != nil {
				ui.connection.Logger.Printf("handleMoveEvents: PlayNextTrack -- %s", err.Error())
			}
		} else if e.Event_Id == mpv.EVENT_START_FILE {
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
						ui.connection.Logger.Printf("scrobbler: timer started, %v", scrobbleDuration)
					} else {
						ui.connection.Logger.Printf("scrobbler: track too short")
					}
				}
			}
		} else if e.Event_Id == mpv.EVENT_IDLE || e.Event_Id == mpv.EVENT_NONE {
			continue
		}

		position, err := ui.player.Instance.GetProperty("time-pos", mpv.FORMAT_DOUBLE)
		if err != nil {
			ui.connection.Logger.Printf("handleMoveEvents (%s): GetProperty %s -- %s", e.Event_Id.String(), "time-pos", err.Error())
		}
		// TODO only update these as needed
		duration, err := ui.player.Instance.GetProperty("duration", mpv.FORMAT_DOUBLE)
		if err != nil {
			ui.connection.Logger.Printf("handleMoveEvents (%s): GetProperty %s -- %s", e.Event_Id.String(), "duration", err.Error())
		}
		volume, err := ui.player.Instance.GetProperty("volume", mpv.FORMAT_INT64)
		if err != nil {
			ui.connection.Logger.Printf("handleMoveEvents (%s): GetProperty %s -- %s", e.Event_Id.String(), "volume", err.Error())
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

func makeModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewGrid().
		SetColumns(0, width, 0).
		SetRows(0, height, 0).
		AddItem(p, 1, 1, 1, 1, 0, 0, true)
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
