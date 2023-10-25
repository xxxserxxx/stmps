package main

import (
	"time"
)

type eventLoop struct {
	scrobbleTimer *time.Timer
}

func (ui *Ui) runEventLoops() {
	el := &eventLoop{}
	ui.eventLoop = el

	// create reused timer to scrobble after delay
	el.scrobbleTimer = time.NewTimer(0)
	if !el.scrobbleTimer.Stop() {
		<-el.scrobbleTimer.C
	}

	go ui.guiEventLoop()
	go ui.backgroundEventLoop()
}

func (ui *Ui) guiEventLoop() {
	ui.addStarredToList()

	//lint:ignore S1000 // additional cases may be added later
	//nolint:gosimple
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
		}
	}
}

// loop for blocking background tasks that would otherwise block the ui
func (ui *Ui) backgroundEventLoop() {
	//lint:ignore S1000 // additional cases may be added later
	//nolint:gosimple
	for {
		select {
		case <-ui.eventLoop.scrobbleTimer.C:
			// scrobble submission delay elapsed
			paused, err := ui.player.IsPaused()
			ui.logger.Printf("scrobbler event: paused %v, err %v, qlen %d", paused, err, len(ui.player.Queue))

			isPlaying := err == nil && !paused
			// TODO we need some mutexed thing to access ui data
			if len(ui.player.Queue) > 0 && isPlaying {
				// it's still playing, submit it
				currentSong := ui.player.Queue[0]
				ui.connection.ScrobbleSubmission(currentSong.Id, true)
			}
		}
	}
}
