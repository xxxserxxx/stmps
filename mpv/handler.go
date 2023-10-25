package mpv

import (
	"github.com/wildeyedskies/go-mpv/mpv"
)

func (p *Player) EventLoop() {
	p.instance.ObserveProperty(0, "time-pos", mpv.FORMAT_DOUBLE)
	p.instance.ObserveProperty(0, "duration", mpv.FORMAT_DOUBLE)
	p.instance.ObserveProperty(0, "volume", mpv.FORMAT_INT64)

	for evt := range p.mpvEvents {
		if evt == nil {
			// quit signal
			break
		} else if evt.Event_Id == mpv.EVENT_END_FILE && !p.replaceInProgress {
			// we don't want to update anything if we're in the process of replacing the current track

			/* TODO do we want this?
			if len(p.queue) > 0 {
				p.queue = p.queue[1:]
			}

			if err := p.PlayNextTrack(); err != nil {
				p.logger.Printf("handleMpvEvents: PlayNextTrack -- %s", err.Error())
			}*/

			p.sendGuiEvent(EventStopped)
		} else if evt.Event_Id == mpv.EVENT_START_FILE {
			p.replaceInProgress = false

			currentSong := QueueItem{}
			if len(p.queue) > 0 {
				currentSong = p.queue[0]
			}
			p.sendGuiDataEvent(EventPlaying, currentSong)
		} else if evt.Event_Id == mpv.EVENT_IDLE || evt.Event_Id == mpv.EVENT_NONE {
			continue
		}

		position, err := p.instance.GetProperty("time-pos", mpv.FORMAT_DOUBLE)
		if err != nil {
			p.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "time-pos", err.Error())
		}
		// TODO only update these as needed
		duration, err := p.instance.GetProperty("duration", mpv.FORMAT_DOUBLE)
		if err != nil {
			p.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "duration", err.Error())
		}
		volume, err := p.instance.GetProperty("volume", mpv.FORMAT_INT64)
		if err != nil {
			p.logger.Printf("handleMpvEvents (%s): GetProperty %s -- %s", evt.Event_Id.String(), "volume", err.Error())
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

		statusData := StatusData{
			Volume:   volume.(int64),
			Position: position.(float64),
			Duration: duration.(float64),
		}
		p.sendGuiDataEvent(EventStatus, statusData)
	}
}

func (p *Player) sendGuiEvent(typ UiEventType) {
	if p.eventConsumer == nil {
		return
	}
	p.eventConsumer.SendEvent(UiEvent{
		Type: typ,
		Data: nil,
	})
}

func (p *Player) sendGuiDataEvent(typ UiEventType, data interface{}) {
	if p.eventConsumer == nil {
		return
	}
	p.eventConsumer.SendEvent(UiEvent{
		Type: typ,
		Data: data,
	})
}
