package logger

type LoggerInterface interface {
	Print(s string)
	Printf(s string, as ...interface{})
	PrintError(source string, err error)
}
