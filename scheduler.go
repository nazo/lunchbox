package lunchbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/nazo/lunchbox/notifier"
	"github.com/robfig/cron"
)

type Scheduler interface {
	Start(ctx context.Context, dags []*Dag)
	SetKeyPrefix(prefix string)
}

type BasicScheduler struct {
	Scheduler
	redisClient *redis.Client
	keyPrefix   string
	notifiers   []notifier.Notifier
}

type NextTask struct {
	Time   time.Time
	DagID  string
	TaskID string
}

func NewScheduler(notifiers []notifier.Notifier, redisClient *redis.Client) Scheduler {
	return &BasicScheduler{
		redisClient: redisClient,
		keyPrefix:   "lunchbox",
		notifiers:   notifiers,
	}
}

func getNextActionTimes(schedule cron.Schedule, lastTime *time.Time, currentTime *time.Time) []*time.Time {
	var times []*time.Time
	checkTime := *lastTime
	for {
		nextTime := schedule.Next(checkTime)
		if nextTime.After(*currentTime) {
			break
		}
		times = append(times, &nextTime)
		checkTime = nextTime
	}
	return times
}

func SchedulerKey(prefix string, dagID string) string {
	return fmt.Sprintf("%s:lasttime:%s", prefix, dagID)
}

func (s *BasicScheduler) SetKeyPrefix(prefix string) {
	s.keyPrefix = prefix
}

func (s *BasicScheduler) Start(ctx context.Context, dags []*Dag) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().Truncate(time.Minute)
		for _, dag := range dags {
			lastTimeString, err := s.redisClient.GetSet(ctx, SchedulerKey(s.keyPrefix, dag.ID), now.Format(time.RFC3339)).Result()
			if err == redis.Nil {
				continue
			} else if err != nil {
				log.Fatalln(err)
			}
			lastTime, err := time.Parse(time.RFC3339, lastTimeString)
			if err != nil {
				log.Fatalln(err)
			}
			actionTimes := getNextActionTimes(dag.Schedule, &lastTime, &now)
			for _, actionTime := range actionTimes {
				taskJSON, err := json.Marshal(&NextTask{
					Time:   *actionTime,
					DagID:  dag.ID,
					TaskID: uuid.New().String(),
				})
				if err != nil {
					log.Fatalln(err)
				}
				err = s.redisClient.LPush(ctx, WorkerKey(s.keyPrefix), taskJSON).Err()
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
	}
}
