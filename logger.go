package main

import (
	"log"
	"log/slog"
	"os"
)

type SlogLogger struct {
	log *slog.Logger
}

func NewSlogLogger() Logger {
	sl := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return &SlogLogger{log: sl}
}
func (sl *SlogLogger) Debug(msg string, keysAndValues ...interface{}) {
	sl.log.Debug(msg, keysAndValues...)
}

func (sl *SlogLogger) Info(msg string, keysAndValues ...interface{}) {
	sl.log.Info(msg, keysAndValues...)
}

func (sl *SlogLogger) Warn(msg string, keysAndValues ...interface{}) {
	sl.log.Warn(msg, keysAndValues...)
}

func (sl *SlogLogger) Error(msg string, keysAndValues ...interface{}) {
	sl.log.Error(msg, keysAndValues...)
}

func (sl *SlogLogger) Fatal(msg string, keysAndValues ...interface{}) {
	sl.log.Error(msg, keysAndValues...)
	log.Fatal(msg)
}

type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Fatal(msg string, keysAndValues ...interface{})
}
