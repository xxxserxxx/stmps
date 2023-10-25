package mpv

type UiEventType int

const (
	// song stopped at end of queue, data: nil
	EventStopped UiEventType = iota
	// new song started playing, data: QueueItem
	EventPlaying
	// UI status update, data: StatusData
	EventStatus
)

type UiEvent struct {
	Type UiEventType
	Data interface{}
}

type EventConsumer interface {
	// create event that goes from mpv backend (this package) to a UI frontend
	SendEvent(event UiEvent)
}
