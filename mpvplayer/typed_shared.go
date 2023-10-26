package mpvplayer

type QueueItem struct {
	Id       string
	Uri      string
	Title    string
	Artist   string
	Duration int
}

// StatusData is a player progress report for the UI
type StatusData struct {
	Volume   int64
	Position float64
	Duration float64
}
