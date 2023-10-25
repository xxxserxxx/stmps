package logger

type LoggerInterface interface {
	Printf(s string, as ...interface{})
}
