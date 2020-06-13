# lunchbox

AWS ECS cron like task scheduler.

** EXPERIMENTAL **

# requirement

- Redis

# quick start

add config.yml

```
Redis:
  KeyPrefix: lunchbox
  Options:
    Address: localhost:6379
    Password: ""
    DB: 0
Notification:
  -
    Driver: slack
    Slack:
      Token: [slack token]
      Channel: [slack channel]
```

add dag.yml on dags/ folder

```
Cron: "10 * * * *"
Cluster: [ecs cluster name]
TaskDefinition: [ecs task definition]
LaunchType: FARGATE
NetworkConfiguration:
  AwsvpcConfiguration:
    AssignPublicIp: ENABLED
    SecurityGroups:
      - [security group id]
    Subnets: 
      - [vpc subnet id]
```

and run.
