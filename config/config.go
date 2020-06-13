package config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/go-redis/redis/v8"
	"sigs.k8s.io/yaml"
)

type ConfigNotificationSlack struct {
	Token   string
	Channel string
}

type ConfigNotification struct {
	Driver string
	Slack  *ConfigNotificationSlack
}

type ConfigRedis struct {
	KeyPrefix string
	Options   *redis.Options
}

type Config struct {
	Redis        *ConfigRedis
	Notification []*ConfigNotification
}

func getConfigFilename() (string, error) {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("os.Getwd failed: %s", err)
		}

		configFile = filepath.Join(currentDir, "config.yml")
	}

	return configFile, nil
}

func LoadConfig() (*Config, error) {
	configFilename, err := getConfigFilename()
	if err != nil {
		return nil, fmt.Errorf("getDagDir failed: %s", err)
	}

	data, err := ioutil.ReadFile(configFilename)
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
	config := &Config{}
	err = yaml.Unmarshal(out.Bytes(), config)
	if err != nil {
		return nil, fmt.Errorf("yaml.UnmarshalStrict failed: %s", err)
	}

	return config, nil
}
