package lunchbox

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/robfig/cron"
	"sigs.k8s.io/yaml"
)

type DagSource struct {
	Cron                 string
	Cluster              string
	TaskDefinition       string
	LaunchType           string
	NetworkConfiguration ecs.NetworkConfiguration
	Overrides            ecs.TaskOverride
}

type Dag struct {
	Id       string
	Source   *DagSource
	Schedule cron.Schedule
}

func readDag(file string) (*DagSource, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadFile failed: %s", err)
	}
	t, err := template.New("").Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("template.Parse failed: %s", err)
	}
	out := new(bytes.Buffer)
	if err := t.Execute(out, nil); err != nil {
		return nil, fmt.Errorf("template.Execute failed: %s", err)
	}
	dag := &DagSource{}
	err = yaml.Unmarshal(out.Bytes(), dag)
	if err != nil {
		return nil, fmt.Errorf("yaml.UnmarshalStrict failed: %s", err)
	}

	return dag, nil
}

func getDagDir() (string, error) {
	dagDir := os.Getenv("DAG_DIR")
	if dagDir == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("os.Getwd failed: %s", err)
		}

		dagDir = filepath.Join(currentDir, "dags")
	}

	return dagDir, nil
}

func LoadFromTaskDir() ([]*Dag, error) {
	dagDir, err := getDagDir()
	if err != nil {
		return nil, fmt.Errorf("getDagDir failed: %s", err)
	}
	files, err := ioutil.ReadDir(dagDir)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadDir failed: %s", err)
	}

	var dags []*Dag
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".yaml") {
			source, err := readDag(filepath.Join(dagDir, file.Name()))
			if err != nil {
				return nil, fmt.Errorf("readDag failed: %s", err)
			}
			schedule, err := cronParser.Parse(source.Cron)
			if err != nil {
				return nil, fmt.Errorf("cronParser.Parse failed: %s", err)
			}
			dags = append(dags, &Dag{
				Source:   source,
				Schedule: schedule,
			})
		}
	}

	if len(dags) == 0 {
		return nil, fmt.Errorf("no dags found.")
	}

	return dags, nil
}
