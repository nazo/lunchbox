package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/go-redis/redis/v8"
	"github.com/nazo/lunchbox"
	configLoader "github.com/nazo/lunchbox/config"
	"github.com/nazo/lunchbox/notifier"
	notifierFactory "github.com/nazo/lunchbox/notifier/factory"
)

func main() {
	dags, err := lunchbox.LoadFromTaskDir()
	if err != nil {
		log.Fatal(err)
	}

	session := session.Must(session.NewSession())
	config, err := configLoader.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	ecsService := ecs.New(session)
	cloudwatchlogsService := cloudwatchlogs.New(session)
	redisClient := redis.NewClient(config.Redis.Options)

	var notifiers []notifier.Notifier
	for _, notificatonConfig := range config.Notification {
		notifierImpl := notifierFactory.New(notificatonConfig)
		if notifierImpl == nil {
			continue
		}
		notifiers = append(notifiers, notifierImpl)
	}

	go lunchbox.NewScheduler(notifiers, redisClient).Start(context.Background(), dags)
	go lunchbox.NewWorker(notifiers, redisClient, ecsService).Start(context.Background(), dags)
	go lunchbox.NewChecker(notifiers, redisClient, ecsService, cloudwatchlogsService).Start(context.Background(), dags)

	for {
		time.Sleep(1000)
	}
}
