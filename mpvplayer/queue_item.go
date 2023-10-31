package mpvplayer

import "github.com/wildeyedskies/stmp/remote"

var _ remote.TrackInterface = (*QueueItem)(nil)

func (q *QueueItem) GetArtist() string {
	if q == nil {
		return ""
	}
	return q.Artist
}

func (q *QueueItem) GetTitle() string {
	if q == nil {
		return ""
	}
	return q.Title
}

func (q *QueueItem) GetDuration() int {
	if q == nil {
		return 0
	}
	return q.Duration
}

func (q *QueueItem) IsValid() bool {
	return q != nil && q.Id != ""
}
