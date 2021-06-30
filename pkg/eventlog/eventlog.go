// +build windows

package eventlog

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"golang.org/x/sys/windows/svc/eventlog"
)

// EventLogHook to send logs via windows log.
type EventLogHook struct {
	upstream *eventlog.Log
}

// NewHook creates and returns a new EventLogHook wrapped around anything that implements the debug.Log interface
func NewHook(logger *eventlog.Log) *EventLogHook {
	return &EventLogHook{upstream: logger}
}

func (hook *EventLogHook) Fire(entry *logrus.Entry) error {
	var line string
	var err error

	line, err = entry.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	switch entry.Level {
	case logrus.PanicLevel:
		fallthrough
	case logrus.FatalLevel:
		fallthrough
	case logrus.ErrorLevel:
		err = hook.upstream.Error(102, line)
	case logrus.WarnLevel:
		err = hook.upstream.Warning(101, line)
	case logrus.InfoLevel:
		err = hook.upstream.Info(100, line)
	case logrus.DebugLevel:
		err = hook.upstream.Info(100, line)
	}

	if err != nil {
		logrus.WithError(err).Error("unable to send eventlog")
	}

	return err
}

func (hook *EventLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
