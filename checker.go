package lunchbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/go-redis/redis/v8"
	"github.com/nazo/lunchbox/notifier"
)

type FailureDetail struct {
	Logs string
}

type Checker interface {
	Start(ctx context.Context, dags []*Dag)
	SetKeyPrefix(prefix string)
}

type BasicChecker struct {
	Checker
	redisClient           *redis.Client
	ecsService            *ecs.ECS
	cloudwatchlogsService *cloudwatchlogs.CloudWatchLogs
	keyPrefix             string
	notifiers             []notifier.Notifier
}

func NewChecker(notifiers []notifier.Notifier, redisClient *redis.Client, ecsService *ecs.ECS, cloudwatchlogsService *cloudwatchlogs.CloudWatchLogs) Checker {
	return &BasicChecker{
		redisClient:           redisClient,
		ecsService:            ecsService,
		cloudwatchlogsService: cloudwatchlogsService,
		keyPrefix:             "lunchbox",
		notifiers:             notifiers,
	}
}

func CheckerKey(prefix string) string {
	return fmt.Sprintf("%s:running", prefix)
}

func (c *BasicChecker) SetKeyPrefix(prefix string) {
	c.keyPrefix = prefix
}

func buildCloudWatchLogStreamName(containerDef *ecs.ContainerDefinition, task *ecs.Task) string {
	taskArnSplit := strings.Split(*task.TaskArn, "/")
	return fmt.Sprintf("%s/%s/%s", *containerDef.LogConfiguration.Options["awslogs-stream-prefix"], *containerDef.Name, taskArnSplit[1])
}

func (c *BasicChecker) getLog(containerDef *ecs.ContainerDefinition, task *ecs.Task) (string, error) {
	if *containerDef.LogConfiguration.LogDriver != "awslogs" {
		return "log driver not supported", nil
	}
	getLogEventsOutput, err := c.cloudwatchlogsService.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		Limit:         aws.Int64(50),
		LogGroupName:  containerDef.LogConfiguration.Options["awslogs-group"],
		LogStreamName: aws.String(buildCloudWatchLogStreamName(containerDef, task)),
	})
	if err != nil {
		return "", err
	}
	log := ""
	for _, event := range getLogEventsOutput.Events {
		log = fmt.Sprintf("%s%s\n", log, *event.Message)
	}
	return log, nil
}

func (c *BasicChecker) watchTask(ctx context.Context, taskRun *TaskRun) {
	describeTasksOutput, err := c.ecsService.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: aws.String(*taskRun.Task.ClusterArn),
		Tasks:   aws.StringSlice([]string{*taskRun.Task.TaskArn}),
	})
	if err != nil {
		log.Fatalln(err)
		return
	}
	if *describeTasksOutput.Tasks[0].LastStatus != "STOPPED" {
		taskRunJSON, err := json.Marshal(taskRun)
		if err != nil {
			log.Fatalln(err)
		}
		err = c.redisClient.RPush(ctx, CheckerKey(c.keyPrefix), taskRunJSON).Err()
		if err != nil {
			log.Fatalln(err)
		}
		return
	}

	logOutput := &notifier.LogOutput{
		Status:        *describeTasksOutput.Tasks[0].LastStatus,
		StoppingAt:    *describeTasksOutput.Tasks[0].StoppingAt,
		StoppedAt:     *describeTasksOutput.Tasks[0].StoppedAt,
		StoppedReason: *describeTasksOutput.Tasks[0].StoppedReason,
		ContainerLogs: map[string]*notifier.ContainerLog{},
	}
	fmt.Printf("%s\n", *describeTasksOutput.Tasks[0].StoppedReason)
	for _, containerError := range describeTasksOutput.Tasks[0].Containers {
		if containerError.Reason != nil {
			logOutput.ContainerLogs[*containerError.Name] = &notifier.ContainerLog{
				ShortMessage: *containerError.Reason,
			}
		}
	}
	for _, containerDef := range taskRun.TaskDefinition.ContainerDefinitions {
		longLog, err := c.getLog(containerDef, taskRun.Task)
		if err != nil {
			log.Println("getLog failed")
			log.Fatalln(err)
			return
		}
		name := *containerDef.Name
		if _, ok := logOutput.ContainerLogs[name]; !ok {
			logOutput.ContainerLogs[name] = &notifier.ContainerLog{}
		}
		logOutput.ContainerLogs[name].Log = longLog
	}
	for _, notifierImpl := range c.notifiers {
		err = notifierImpl.Notify(logOutput)
		if err != nil {
			log.Fatalln(err)
			return
		}
	}
}

func (c *BasicChecker) Start(ctx context.Context, dags []*Dag) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for {
			taskJSON, err := c.redisClient.LPop(ctx, CheckerKey(c.keyPrefix)).Result()
			if err == redis.Nil {
				break
			} else if err != nil {
				log.Fatalln(err)
			}
			taskRun := &TaskRun{}
			err = json.Unmarshal([]byte(taskJSON), taskRun)
			if err != nil {
				log.Fatalln(err)
			}
			c.watchTask(ctx, taskRun)
		}
	}
}
