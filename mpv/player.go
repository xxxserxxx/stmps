package mpv

import (
	"errors"
	"strconv"

	"github.com/wildeyedskies/go-mpv/mpv"
	"github.com/wildeyedskies/stmp/logger"
)

const (
	// TODO make private?
	PlayerStopped = iota
	PlayerPlaying
	PlayerPaused
	PlayerError
)

type PlayerQueue []QueueItem

type Player struct {
	instance          *mpv.Mpv
	mpvEvents         chan *mpv.Event
	eventConsumer     EventConsumer
	queue             PlayerQueue
	logger            logger.LoggerInterface
	replaceInProgress bool
}

func eventListener(m *mpv.Mpv) chan *mpv.Event {
	c := make(chan *mpv.Event)
	go func() {
		for {
			e := m.WaitEvent(1)
			c <- e
		}
	}()
	return c
}

func NewPlayer(logger logger.LoggerInterface) (player *Player, err error) {
	mpvInstance := mpv.Create()

	// TODO figure out what other mpv options we need
	mpvInstance.SetOptionString("audio-display", "no")
	mpvInstance.SetOptionString("video", "no")

	if err = mpvInstance.Initialize(); err != nil {
		mpvInstance.TerminateDestroy()
		return
	}

	player = &Player{
		instance:          mpvInstance,
		mpvEvents:         eventListener(mpvInstance),
		eventConsumer:     nil, // must be set by calling RegisterEventConsumer()
		queue:             make([]QueueItem, 0),
		logger:            logger,
		replaceInProgress: false,
	}
	return
}

func (p *Player) Quit() {
	p.mpvEvents <- nil
	p.instance.TerminateDestroy()
}

func (p *Player) RegisterEventConsumer(consumer EventConsumer) {
	p.eventConsumer = consumer
}

func (p *Player) PlayNextTrack() error {
	if len(p.queue) > 0 {
		return p.instance.Command([]string{"loadfile", p.queue[0].Uri})
	}
	return nil
}

func (p *Player) Play(id string, uri string, title string, artist string, duration int) error {
	p.queue = []QueueItem{{id, uri, title, artist, duration}}
	p.replaceInProgress = true
	if ip, e := p.IsPaused(); ip && e == nil {
		p.Pause()
	}
	return p.instance.Command([]string{"loadfile", uri})
}

func (p *Player) Stop() error {
	return p.instance.Command([]string{"stop"})
}

func (p *Player) IsSongLoaded() (bool, error) {
	idle, err := p.instance.GetProperty("idle-active", mpv.FORMAT_FLAG)
	return !idle.(bool), err
}

func (p *Player) IsPaused() (bool, error) {
	pause, err := p.instance.GetProperty("pause", mpv.FORMAT_FLAG)
	return pause.(bool), err
}

// Pause toggles playing music
// If a song is playing, it is paused. If a song is paused, playing resumes. The
// state after the toggle is returned, or an error.
func (p *Player) Pause() (err error) {
	loaded, err := p.IsSongLoaded()
	if err != nil {
		return
	}
	paused, err := p.IsPaused()
	if err != nil {
		return
	}

	if loaded {
		err = p.instance.Command([]string{"cycle", "pause"})
		if err != nil {
			return
		}
		paused = !paused

		currentSong := QueueItem{}
		if len(p.queue) > 0 {
			currentSong = p.queue[0]
		}

		if paused {
			p.sendGuiDataEvent(EventPaused, currentSong)
		} else {
			p.sendGuiDataEvent(EventUnpaused, currentSong)
		}
	} else {
		if len(p.queue) > 0 {
			err = p.instance.Command([]string{"loadfile", p.queue[0].Uri})
			if err != nil {
				return
			}

			p.sendGuiDataEvent(EventUnpaused, p.queue[0])
		} else {
			p.sendGuiEvent(EventStopped)
		}
	}

	return
}

func (p *Player) SetVolume(percentValue int64) error {
	if percentValue > 100 {
		percentValue = 100
	} else if percentValue < 0 {
		percentValue = 0
	}

	return p.instance.SetProperty("volume", mpv.FORMAT_INT64, percentValue)
}

func (p *Player) AdjustVolume(increment int64) error {
	volume, err := p.instance.GetProperty("volume", mpv.FORMAT_INT64)
	if err != nil {
		return err
	}
	if volume == nil {
		return nil
	}

	return p.SetVolume(volume.(int64) + increment)
}

func (p *Player) Volume() (int64, error) {
	volume, err := p.instance.GetProperty("volume", mpv.FORMAT_INT64)
	if err != nil {
		return -1, err
	}
	return volume.(int64), nil
}

func (p *Player) Seek(increment int) error {
	return p.instance.Command([]string{"seek", strconv.Itoa(increment)})
}

// accessed from gui context
func (p *Player) ClearQueue() {
	p.queue = make([]QueueItem, 0) // TODO mutex queue access
}

func (p *Player) DeleteQueueItem(index int) {
	// TODO mutex queue access
	if len(p.queue) > 1 {
		p.queue = append(p.queue[:index], p.queue[index+1:]...)
	} else {
		p.queue = make([]QueueItem, 0)
	}
}

func (p *Player) AddToQueue(item *QueueItem) {
	p.queue = append(p.queue, *item)
}

func (p *Player) GetQueueItem(index int) (QueueItem, error) {
	if index < 0 || index >= len(p.queue) {
		return QueueItem{}, errors.New("invalid queue entry")
	}
	return p.queue[index], nil
}

func (p *Player) GetQueueCopy() PlayerQueue {
	cpy := make(PlayerQueue, len(p.queue))
	copy(cpy, p.queue)
	return cpy
}

// accessed from background context
func (p *Player) GetPlayingTrack() (QueueItem, error) {
	paused, err := p.IsPaused()
	if err != nil {
		return QueueItem{}, err
	}
	if paused {
		return QueueItem{}, errors.New("not playing")
	}

	if len(p.queue) == 0 { // TODO mutex queue access
		return QueueItem{}, errors.New("queue empty")
	}
	currentSong := p.queue[0]
	return currentSong, nil
}
