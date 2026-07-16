// Package log provides per-component scoped logging.
package log

import (
	"fmt"
	"time"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

var globalLevel Level = Info

func SetDefaultLevel(level Level) {
	globalLevel = level
}

type Logger struct {
	component string
}

func For(component string) Logger {
	return Logger{
		component: component,
	}
}

type record struct {
	level Level
	msg   string
}

func (l Logger) print(r record) {
	if r.level < globalLevel {
		return
	}

	var lvl string
	switch r.level {
	case Debug:
		lvl = "DEBUG"
	case Info:
		lvl = "INFO "
	case Warn:
		lvl = "WARN "
	case Error:
		lvl = "ERROR"
	}

	parts := []any{
		time.Now().Format(time.DateTime),
		lvl,
		l.component,
		"|",
		r.msg,
	}

	fmt.Println(parts...)
}

func (l Logger) Debug(s string) {
	l.print(record{level: Debug, msg: s})
}

func (l Logger) Debugf(format string, a ...any) {
	l.Debug(fmt.Sprintf(format, a...))
}

func (l Logger) Info(s string) {
	l.print(record{level: Info, msg: s})
}

func (l Logger) Infof(format string, a ...any) {
	l.Info(fmt.Sprintf(format, a...))
}

func (l Logger) Warn(s string) {
	l.print(record{level: Warn, msg: s})
}

func (l Logger) Warnf(format string, a ...any) {
	l.Warn(fmt.Sprintf(format, a...))
}

func (l Logger) Error(s string) {
	l.print(record{level: Error, msg: s})
}

func (l Logger) Errorf(format string, a ...any) {
	l.Error(fmt.Sprintf(format, a...))
}

func (l Logger) ErrorV(err error) {
	l.Error(err.Error())
}
