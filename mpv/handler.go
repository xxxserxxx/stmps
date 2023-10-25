package mpv

import (
	"fmt"
	"math"
	"time"

	"github.com/wildeyedskies/go-mpv/mpv"
)

func (p *Player) EventLoop() {
	p.Instance.ObserveProperty(0, "time-pos", mpv.FORMAT_DOUBLE)
	p.Instance.ObserveProperty(0, "duration", mpv.FORMAT_DOUBLE)
	p.Instance.ObserveProperty(0, "volume", mpv.FORMAT_INT64)

	for evt := range p.EventChannel {
		if evt == nil {
			// quit signal
			break
		} else if evt.Event_Id == mpv.EVENT_END_FILE && !p.ReplaceInProgress {
			// we don't want to update anything if we're in the process of replacing the current track
			ui.startStopStatus.SetText("[::b]stmp: [red]stopped")

			// TODO it's gross that this is here, need better event handling
			if len(p.Queue) > 0 {
				p.Queue = p.Queue[1:]
			}
			updateQueueList(p, ui.queueList, ui.starIdList)
			err := p.PlayNextTrack()
			if err != nil {
				ui.logger.Printf("handleMpvEvents: PlayNextTrack -- %s", err.Error())
			}
		} else if evt.Event_Id == mpv.EVENT_START_FILE {
			p.ReplaceInProgress = false
			updateQueueList(p, ui.queueList, ui.starIdList)

			if len(p.Queue) > 0 {
				currentSong := p.Queue[0]
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

						// HACK
						ui.eventLoop.scrobbleTimer.Reset(scrobbleDuration)
						ui.logger.Printf("scrobbler: timer started, %v", scrobbleDuration)
					} else {
						ui.logger.Printf("scrobbler: track too short")
					}
				}
			}
		} else if evt.Event_Id == mpv.EVENT_IDLE || evt.Event_Id == mpv.EVENT_NONE {
			continue
		}

		position, err := p.Instance.GetProperty("time-pos", mpv.FORMAT_DOUBLE)
		if err != nil {
			ui.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "time-pos", err.Error())
		}
		// TODO only update these as needed
		duration, err := p.Instance.GetProperty("duration", mpv.FORMAT_DOUBLE)
		if err != nil {
			ui.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "duration", err.Error())
		}
		volume, err := p.Instance.GetProperty("volume", mpv.FORMAT_INT64)
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

		pStatus.SetText(formatPlayerStatus(volume.(int64), position.(float64), duration.(float64)))
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
