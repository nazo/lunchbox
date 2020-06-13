package lunchbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/go-redis/redis/v8"
	"github.com/nazo/lunchbox/notifier"
)

type TaskRun struct {
	TaskDefinition *ecs.TaskDefinition
	Task           *ecs.Task
	dagID          string
}

type Worker interface {
	Start(ctx context.Context, dags []*Dag)
	StartWorker(ctx context.Context, dags []*Dag)
	StartFollower(ctx context.Context, dags []*Dag)
	SetKeyPrefix(prefix string)
}

func WorkerKey(prefix string) string {
	return fmt.Sprintf("%s:queue", prefix)
}

func WorkerProsessKey(prefix string, taskID string) string {
	return fmt.Sprintf("%s:process:%s", prefix, taskID)
}

func BackupKey(prefix string) string {
	return fmt.Sprintf("%s:queue-backup", prefix)
}

type BasicWorker struct {
	Worker
	redisClient *redis.Client
	ecsService  *ecs.ECS
	keyPrefix   string
	notifiers   []notifier.Notifier
}

func NewWorker(notifiers []notifier.Notifier, redisClient *redis.Client, ecsService *ecs.ECS) Worker {
	return &BasicWorker{
		redisClient: redisClient,
		ecsService:  ecsService,
		keyPrefix:   "lunchbox",
		notifiers:   notifiers,
	}
}

func (s *BasicWorker) SetKeyPrefix(prefix string) {
	s.keyPrefix = prefix
}

func (w *BasicWorker) runTask(ctx context.Context, dag *Dag) {
	fmt.Printf("begin dag %s\n", dag.Source.TaskDefinition)
	listTaskDefinitionsInput := &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: &dag.Source.TaskDefinition,
		Sort:         aws.String("DESC"),
		MaxResults:   aws.Int64(1),
	}
	listTaskDefintionsResult, err := w.ecsService.ListTaskDefinitions(listTaskDefinitionsInput)
	if err != nil {
		log.Fatalln(err)
		return
	}
	if len(listTaskDefintionsResult.TaskDefinitionArns) == 0 {
		log.Fatalln("no task defintions")
		return
	}

	taskDefArn := listTaskDefintionsResult.TaskDefinitionArns[0]
	taskDefValue := strings.Split(*taskDefArn, "/")
	runTaskInput := &ecs.RunTaskInput{
		Cluster:              aws.String(dag.Source.Cluster),
		TaskDefinition:       aws.String(taskDefValue[1]),
		LaunchType:           aws.String(dag.Source.LaunchType),
		NetworkConfiguration: &dag.Source.NetworkConfiguration,
		Overrides:            &dag.Source.Overrides,
	}
	runTaskOutput, err := w.ecsService.RunTask(runTaskInput)
	if err != nil {
		log.Fatalln(err)
		return
	}
	fmt.Printf("start task %s\n", taskDefValue)

	describeTaskDefinitionsOutput, err := w.ecsService.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefValue[1]),
	})
	if err != nil {
		log.Fatalln(err)
		return
	}

	taskRun := &TaskRun{
		TaskDefinition: describeTaskDefinitionsOutput.TaskDefinition,
		Task:           runTaskOutput.Tasks[0],
		dagID:          dag.Id,
	}
	fmt.Printf("%+v\n", taskRun)

	taskRunJson, err := json.Marshal(taskRun)
	if err != nil {
		log.Fatalln(err)
	}
	err = w.redisClient.RPush(ctx, CheckerKey(w.keyPrefix), taskRunJson).Err()
	if err != nil {
		log.Fatalln(err)
	}
}

func findDagById(dags []*Dag, ID string) *Dag {
	for _, dag := range dags {
		if dag.Id == ID {
			return dag
		}
	}

	return nil
}

func (w *BasicWorker) StartWorker(ctx context.Context, dags []*Dag) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for {
			taskJson, err := w.redisClient.RPopLPush(ctx, WorkerKey(w.keyPrefix), BackupKey(w.keyPrefix)).Result()
			if err == redis.Nil {
				break
			} else if err != nil {
				log.Fatalln(err)
			}
			task := &NextTask{}
			err = json.Unmarshal([]byte(taskJson), task)
			if err != nil {
				log.Fatalln(err)
			}
			err = w.redisClient.Set(ctx, WorkerProsessKey(w.keyPrefix, task.TaskID), time.Now(), 0).Err()
			if err != nil {
				log.Fatalln(err)
			}
			dag := findDagById(dags, task.DagID)
			w.runTask(ctx, dag)
		}
	}
}

func (w *BasicWorker) StartFollower(ctx context.Context, dags []*Dag) {
}

func (w *BasicWorker) Start(ctx context.Context, dags []*Dag) {
	go w.StartWorker(ctx, dags)
	go w.StartFollower(ctx, dags)
}
