# asg-node-refresh

[![Go Report Card](https://goreportcard.com/badge/github.com/xandout/asg-node-refresh)](https://goreportcard.com/badge/github.com/xandout/asg-node-refresh)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xandout/asg-node-refresh/master/LICENSE)


An AWS Auto Scaling Group refresh tool.

## Why?

AWS offers only one way to schedule node refreshes in an ASG: `max_instance_lifetime`.  This method will refresh an instance after the TTL expires but what happens if the node came up during peak load on a Tuesday afternoon? Well your instance will go away at the same time next Tuesday.  Not ideal.

## How does it work?

This is a Kubernetes native application and uses a `ConfigMap` to store state.

The basic workflow is 

1. Create `CronJob` to run the refresh when it makes sense for your business
1. The `Job` starts on one of your nodes and triggers an ASG refresh, storing the refresh-id in the `ConfigMap`
1. When the node the `Job` started on is terminated as part of the refresh, it is restarted on another node and picks up where it left off
1. Eventually all of the nodes are refreshed and the app cleans up after itself.


## How do I deploy it?

Create an IAM user with the following permissions.  Below are two examples of the IAM policy.  The first is restricted to refreshing a single ASG, the second all ASGs(not recommended).

### IAM Policy - Read one ASG
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowStartRefresh",
            "Effect": "Allow",
            "Action": "autoscaling:StartInstanceRefresh",
            "Resource": "ASG_ARN"
        },
        {
            "Sid": "AllowDescribeRefreshes",
            "Effect": "Allow",
            "Action": "autoscaling:DescribeInstanceRefreshes",
            "Resource": "*"
        }
    ]
}
```

### IAM Policy - Read All ASGs
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowStartRefresh",
            "Effect": "Allow",
            "Action": "autoscaling:StartInstanceRefresh",
            "Resource": "arn:aws:autoscaling:*:*:autoScalingGroup:*:autoScalingGroupName/*"
        },
        {
            "Sid": "AllowDescribeRefreshes",
            "Effect": "Allow",
            "Action": "autoscaling:DescribeInstanceRefreshes",
            "Resource": "*"
        }
    ]
}
```

## Update YAML


### Set AWS credentials in [aws_creds_secret.yml](_manifests/aws_creds_secret.yml.example)

You will need to base64 encode the values of `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and the name(not ARN) of your target ASG.

```
echo -n "YOUR_IAM_ID" | base64
echo -n "YOUR_IAM_SECRET" | base64
echo -n "your-asg-name" | base64
```

### Update the `env` in [cronjob.yml](_manifests/cronjob.yml)

- AWS_REGION
    - example "us-east-1"
- AWS_ASG_NAME
    - example "prod-k8s-asg-1"
- INSTANCE_WARMUP
    - default 300

### Apply the CronJob


```shell
kubectl apply -f _manifests/cronjob.yml
```