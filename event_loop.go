// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"sort"
	"time"

	"github.com/spezifisch/stmps/mpvplayer"
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
	events := 0.0
	fpsTimer := time.NewTimer(0)

	for {
		events++

		select {
		case <-fpsTimer.C:
			fpsTimer.Reset(10 * time.Second)
			// ui.logger.Printf("guiEventLoop: %f events per second", events/10.0)
			events = 0

		case msg := <-ui.logger.Prints:
			// handle log page output
			ui.logPage.Print(msg)

		case mpvEvent := <-ui.mpvEvents:
			events++

			// handle events from mpv wrapper
			switch mpvEvent.Type {
			case mpvplayer.EventStatus:
				if mpvEvent.Data == nil {
					continue
				}
				// TODO (E) is mpvEvent.Data thread-safe? maybe we need a copy
				statusData := mpvEvent.Data.(mpvplayer.StatusData)
				if ui.scanning {
					scanning, err := ui.connection.ScanStatus()
					if err != nil {
						ui.logger.PrintError("ScanStatus", err)
					}
					ui.scanning = scanning.Scanning
				}

				ui.app.QueueUpdateDraw(func() {
					txt := formatPlayerStatus(ui.scanning, statusData.Volume, statusData.Position, statusData.Duration)
					ui.playerStatus.SetText(txt)
					if ui.queuePage.lyrics != nil {
						cl := ui.queuePage.currentLyrics.Lines
						lcl := len(cl)
						if lcl == 0 {
							ui.queuePage.lyrics.SetText("\n[::i]No lyrics[-:-:-]")
						} else {
							// We only get an update every second or so, and Position is truncated
							// to seconds. Make sure that, by the time our tick comes, we're already showing
							// the lyric that's being sung. Do this by pretending that we're a half-second
							// in the future
							p := statusData.Position*1000 + 500
							_, _, _, fh := ui.queuePage.lyrics.GetInnerRect()
							i := sort.Search(len(cl), func(i int) bool {
								return p < cl[i].Start
							})
							if i < lcl && p < cl[i].Start {
								txt := ""
								if i > 1 {
									txt = cl[i-2].Value + "\n"
								}
								if i > 0 {
									txt += "[::b]" + cl[i-1].Value + "[-:-:-]\n"
								}
								for k := i; k < lcl && k-i < fh; k++ {
									txt += cl[k].Value + "\n"
								}
								ui.queuePage.lyrics.SetText(txt)
							}
						}
					}
				})

			case mpvplayer.EventStopped:
				ui.logger.Print("mpvEvent: stopped")
				ui.app.QueueUpdateDraw(func() {
					ui.startStopStatus.SetText("[red::b]Stopped[::-]")
					ui.queuePage.lyrics.SetText("")
					ui.queuePage.updateQueue()
				})

			case mpvplayer.EventPlaying:
				ui.logger.Print("mpvEvent: playing")
				statusText := "[green::b]Playing[::-]"

				var currentSong mpvplayer.QueueItem
				if mpvEvent.Data != nil {
					// TODO (E) is mpvEvent.Data thread safe? maybe we need a copy
					currentSong = mpvEvent.Data.(mpvplayer.QueueItem)
					statusText += formatSongForStatusBar(&currentSong)

					// Update MprisPlayer with new track info
					if ui.mprisPlayer != nil {
						ui.mprisPlayer.OnSongChange(currentSong)
					}

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
					ui.queuePage.updateQueue()
					if len(ui.queuePage.currentLyrics.Lines) == 0 {
						ui.queuePage.lyrics.SetText("\n[::i]No lyrics[-:-:-]")
					} else {
						ui.queuePage.lyrics.SetText("")
					}
				})

			case mpvplayer.EventPaused:
				ui.logger.Print("mpvEvent: paused")
				statusText := "[yellow::b]Paused[::-]"

				var currentSong mpvplayer.QueueItem
				if mpvEvent.Data != nil {
					// TODO mpvEvent.Data thread safe? maybe we need a copy
					currentSong = mpvEvent.Data.(mpvplayer.QueueItem)
					statusText += formatSongForStatusBar(&currentSong)
				}

				ui.app.QueueUpdateDraw(func() {
					ui.startStopStatus.SetText(statusText)
				})

			case mpvplayer.EventUnpaused:
				ui.logger.Print("mpvEvent: unpaused")
				statusText := "[green::b]Playing[::-]"

				var currentSong mpvplayer.QueueItem
				if mpvEvent.Data != nil {
					// TODO is mpvEvent.Data thread safe? maybe we need a copy
					currentSong = mpvEvent.Data.(mpvplayer.QueueItem)
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
			if _, err := ui.connection.ScrobbleSubmission(songId, false); err != nil {
				ui.logger.PrintError("scrobble nowplaying", err)
			}

		case <-ui.eventLoop.scrobbleSubmissionTimer.C:
			// scrobble submission delay elapsed
			if currentSong, err := ui.player.GetPlayingTrack(); err != nil {
				// user paused/stopped
				ui.logger.Printf("not scrobbling: %v", err)
			} else {
				// it's still playing
				ui.logger.Printf("scrobbling: %s", currentSong.Id)
				if _, err := ui.connection.ScrobbleSubmission(currentSong.Id, true); err != nil {
					ui.logger.PrintError("scrobble submission", err)
				}
			}
		}
	}
}

func (ui *Ui) addStarredToList() {
	starred, err := ui.connection.GetStarred()
	if err != nil {
		ui.logger.PrintError("addStarredToList", err)
	}

	for _, e := range starred.Songs {
		// We're storing empty struct as values as we only want the indexes
		// It's faster having direct index access instead of looping through array values
		ui.starIdList[e.Id] = struct{}{}
	}
	for _, e := range starred.Albums {
		ui.starIdList[e.Id] = struct{}{}
	}
	for _, e := range starred.Artists {
		ui.starIdList[e.Id] = struct{}{}
	}
}
