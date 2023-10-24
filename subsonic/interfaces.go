package subsonic

type LoggerInterface interface {
	Printf(s string, as ...interface{})
}
