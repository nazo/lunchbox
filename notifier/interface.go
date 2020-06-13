package notifier

import "time"

type ContainerLog struct {
	Status       string
	ShortMessage string
	Log          string
}

type LogOutput struct {
	ContainerLogs map[string]*ContainerLog
	Status        string
	StopCode      string
	StoppedAt     time.Time
	StoppingAt    time.Time
	StoppedReason string
}

type Notifier interface {
	Notify(logOutput *LogOutput) error
}
