package lunchbox

import (
	"testing"
	"time"

	"github.com/robfig/cron"
)

func TestGetNextActionTimes(t *testing.T) {
	tests := []struct {
		spec, lastTime, currentTime string
		expected                    []string
	}{
		{"* * * * *", "02 Jan 20 10:00 UTC", "02 Jan 20 10:02 UTC", []string{"02 Jan 20 10:01 UTC", "02 Jan 20 10:02 UTC"}},
		{"* * * * *", "02 Jan 20 10:00 UTC", "02 Jan 20 10:00 UTC", []string{}},
		{"19 23 * * *", "02 Jan 20 10:00 UTC", "03 Jan 20 10:02 UTC", []string{"02 Jan 20 23:19 UTC"}},
	}

	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for _, test := range tests {
		schedule, err := cronParser.Parse(test.spec)
		if err != nil {
			t.Errorf("cronParser.Parse failed. spec: %s", test.spec)
		}
		lastTime, err := time.Parse(time.RFC822, test.lastTime)
		if err != nil {
			t.Errorf("time.Parse failed. lastTime: %s", test.lastTime)
		}
		currentTime, err := time.Parse(time.RFC822, test.currentTime)
		if err != nil {
			t.Errorf("time.Parse failed. currenTime: %s", test.currentTime)
		}
		times := getNextActionTimes(schedule, &lastTime, &currentTime)
		if len(times) != len(test.expected) {
			t.Errorf("getNextActionTimes count failed. expected: %+v, actual: %+v", len(test.expected), len(times))
		}
		for key, actual := range times {
			expected, err := time.Parse(time.RFC822, test.expected[key])
			if err != nil {
				t.Errorf("time.Parse failed. expected[%d]: %s", key, test.expected[key])
			}
			if !expected.Equal(*actual) {
				t.Errorf("getNextActionTimes match[%d] failed. expected: %s, actual: %s", key, expected, actual)
			}
		}
	}
}
