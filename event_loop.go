// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"time"

	"github.com/wildeyedskies/stmp/mpvplayer"
)

type eventLoop struct {
	// scrobbles are handled by background loop
	scrobbleNowPlaying      chan string
	scrobbleSubmissionTimer *time.Timer
}

func (ui *Ui) initEventLoops() {
	el := &eventLoop{
		scrobbleNowPlaying: make(chan string, 5),
	}
	ui.eventLoop = el

	// create reused timer to scrobble after delay
	el.scrobbleSubmissionTimer = time.NewTimer(0)
	if !el.scrobbleSubmissionTimer.Stop() {
		<-el.scrobbleSubmissionTimer.C
	}
}

func (ui *Ui) runEventLoops() {
	go ui.guiEventLoop()
	go ui.backgroundEventLoop()
}

// handle ui updates
func (ui *Ui) guiEventLoop() {
	ui.addStarredToList()

	for {
		select {
		case msg := <-ui.logger.Prints:
			// handle log page output
			ui.app.QueueUpdateDraw(func() {
				line := time.Now().Local().Format("(15:04:05) ") + msg
				ui.logList.InsertItem(0, line, "", 0, nil)

				// Make sure the log list doesn't grow infinitely
				for ui.logList.GetItemCount() > 100 {
					ui.logList.RemoveItem(-1)
				}
			})

		case mpvEvent := <-ui.mpvEvents:
			// handle events from mpv wrapper
			switch mpvEvent.Type {
			case mpvplayer.EventStatus:
				if mpvEvent.Data == nil {
					continue
				}
				statusData := mpvEvent.Data.(mpvplayer.StatusData) // TODO is this safe to access? maybe we need a copy

				ui.app.QueueUpdateDraw(func() {
					ui.playerStatus.SetText(formatPlayerStatus(statusData.Volume, statusData.Position, statusData.Duration))
				})

			case mpvplayer.EventStopped:
				ui.logger.Print("mpvEvent: stopped")
				ui.app.QueueUpdateDraw(func() {
					ui.startStopStatus.SetText("[::b]stmp: [red]Stopped")
					updateQueueList(ui.player, ui.queueList, ui.starIdList)
				})

			case mpvplayer.EventPlaying:
				ui.logger.Print("mpvEvent: playing")
				statusText := "[::b]stmp: [green]Playing"

				var currentSong mpvplayer.QueueItem
				if mpvEvent.Data != nil {
					currentSong = mpvEvent.Data.(mpvplayer.QueueItem) // TODO is this safe to access? maybe we need a copy
					statusText += formatSongForStatusBar(&currentSong)

					if ui.connection.Scrobble {
						// scrobble "now playing" event (delegate to background event loop)
						ui.eventLoop.scrobbleNowPlaying <- currentSong.Id

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

							ui.eventLoop.scrobbleSubmissionTimer.Reset(scrobbleDuration)
							ui.logger.Printf("scrobbler: timer started, %v", scrobbleDuration)
						} else {
							ui.logger.Printf("scrobbler: track too short")
						}
					}
				}

				ui.app.QueueUpdateDraw(func() {
					ui.startStopStatus.SetText(statusText)
					updateQueueList(ui.player, ui.queueList, ui.starIdList)
				})

			case mpvplayer.EventPaused:
				ui.logger.Print("mpvEvent: paused")
				statusText := "[::b]stmp: [yellow]Paused"

				var currentSong mpvplayer.QueueItem
				if mpvEvent.Data != nil {
					currentSong = mpvEvent.Data.(mpvplayer.QueueItem) // TODO is this safe to access? maybe we need a copy
					statusText += formatSongForStatusBar(&currentSong)
				}

				ui.app.QueueUpdateDraw(func() {
					ui.startStopStatus.SetText(statusText)
				})

			case mpvplayer.EventUnpaused:
				ui.logger.Print("mpvEvent: unpaused")
				statusText := "[::b]stmp: [green]Playing"

				var currentSong mpvplayer.QueueItem
				if mpvEvent.Data != nil {
					currentSong = mpvEvent.Data.(mpvplayer.QueueItem) // TODO is this safe to access? maybe we need a copy
					statusText += formatSongForStatusBar(&currentSong)
				}

				ui.app.QueueUpdateDraw(func() {
					ui.startStopStatus.SetText(statusText)
				})

			default:
				ui.logger.Printf("guiEventLoop: unhandled mpvEvent %v", mpvEvent)
			}
		}
	}
}

// loop for blocking background tasks that would otherwise block the ui
func (ui *Ui) backgroundEventLoop() {
	for {
		select {
		case songId := <-ui.eventLoop.scrobbleNowPlaying:
			// scrobble now playing
			ui.connection.ScrobbleSubmission(songId, false)

		case <-ui.eventLoop.scrobbleSubmissionTimer.C:
			// scrobble submission delay elapsed
			if currentSong, err := ui.player.GetPlayingTrack(); err != nil {
				// user paused/stopped
				ui.logger.Printf("not scrobbling: %v", err)
			} else {
				// it's still playing
				ui.logger.Printf("scrobbling: %s", currentSong.Id)
				ui.connection.ScrobbleSubmission(currentSong.Id, true)
			}
		}
	}
}
