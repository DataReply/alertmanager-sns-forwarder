# alertmanager-sns-forwarder
###### Dispatching Panic Across the Organization

[![Build Status](https://travis-ci.org/DataReply/alertmanager-sns-forwarder.svg?branch=master)](https://travis-ci.org/DataReply/alertmanager-sns-forwarder)
[![go-doc](https://godoc.org/github.com/DataReply/alertmanager-sns-forwarder?status.svg)](https://godoc.org/github.com/DataReply/alertmanager-sns-forwarder)
[![Go Report Card](https://goreportcard.com/badge/github.com/DataReply/alertmanager-sns-forwarder)](https://goreportcard.com/report/github.com/DataReply/alertmanager-sns-forwarder)
[![Docker Hub](https://img.shields.io/badge/docker-hub-blue.svg?longCache=true&style=hub&logo=docker&label=docker)](https://hub.docker.com/r/datareply/alertmanager-sns-forwarder)


Prometheus [Alertmanager](https://github.com/prometheus/alertmanager) Webhook Receiver for forwarding alerts to AWS SNS. Inspired by https://github.com/inCaller/prometheus_bot.

## Compile

As a Docker image:

```bash
docker build -t alertmanager-sns-forwarder:0.1 .
```

Or as local binary:

```bash
export GOPATH="your go path"
make all
```

[GOPATH related doc](https://golang.org/doc/code.html#GOPATH).

## Usage

1. Build the Docker image.

2. Deploy, preferably on K8s (yaml provided in folder `deploy`).

3. Configure Alertmanager.

### AWS SDK Configuration

The app uses the AWS SDK for Go, so it's necessary to configure it according to the [docs](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html). Specifically this means setting a region and providing credentials.

The region should be set by:

* Setting the AWS_REGION environment variable to the default region.
* Setting the AWS_SDK_LOAD_CONFIG environment variable to true to get the region value from the config file in the .aws/ folder in your home directory.

Alternatively, if you provide the ARN prefix argument (described in the next section), the region will be parsed from the prefix and it's not necessary to provide it.

Accordingly, you can provide credentials by:

* Using IAM roles for Amazon EC2 (recommended if your application is running on an Amazon EC2 instance).
* Using a shared credentials file.
* Using environment variables.

### Arguments

The app accepts some optional arguments, available as flags or env vars.

Flag           | Env Variable           | Default       | Description
---------------|------------------------|---------------|------------
`--addr`        | `SNS_FORWARDER_ADDRESS`     | `9087`             | Address on which to listen.
`--debug`       | `SNS_FORWARDER_DEBUG`       | `false`            | Debug mode
`--arn-prefix`  | `SNS_FORWARDER_ARN_PREFIX`  | not specified      | Prefix to use for SNS topic ARNs. If not specified, will try to be detected automatically.
`--sns-subject` | `SNS_SUBJECT`               | not specified      | Optional parameter to be used as the "Subject" line when the message is delivered to email endpoints.

## Customising messages with template

The app also supports [go templating language](https://golang.org/pkg/text/template/).
The feature was ported from [prometheus_bot](https://github.com/inCaller/prometheus_bot) and you can read more about this functionality in their [docs](https://github.com/inCaller/prometheus_bot#customising-messages-with-template).

The settings for using templates are available as the following flags or env vars.

Flag                         | Env Variable                             | Default       | Description
-----------------------------|------------------------------------------|---------------|------------
`--template-path`            | `SNS_FORWARDER_TEMPLATE_PATH`            |               | Template path
`--template-time-zone`       | `SNS_FORWARDER_TEMPLATE_TIME_ZONE`       |               | Template time zone
`--template-time-out-format` | `SNS_FORWARDER_TEMPLATE_TIME_OUT_FORMAT` |               | Template time out format
`--template-split-token`     | `SNS_FORWARDER_TEMPLATE_SPLIT_TOKEN`     |               | Token used for split measure label

There are also an [example template file](testdata/default.tmpl) along with an [example payload json](testdata/simple.json) provided.

### Endpoints

The app exposes the following HTTP endpoints:

Endpoint         | Method | Description
-----------------|--------|------------
`/alert/<topic>` | `POST` | Endpoint for posting alerts by Alertmanager
`/health`        | `GET`  | Endpoint for k8s readiness and liveness probes
`/metrics`       | `GET`  | Endpoint for Prometheus metrics

### Configuring Alertmanager

Alertmanager configuration file:

```yml
- name: 'sns-forwarder'
  webhook_configs:
  - send_resolved: True
    url: http://<forwarder_url>/alert/<sns_topic_name>
```

Replace `<forwarder_url>` with the correct URL, on K8s using the provided yaml it will be `alertmanager-sns-forwarder-svc.default:9087`.

Replace `<sns_topic_name>` by the name of your SNS topic. You can specify the full ARN, or just the topic name, but then you need to do one of the following:
- deploy the container in AWS, this way the ARN prefix will be detected automatically
- specify the ARN prefix via a parameter `--arn-prefix`
- specify the ARN prefix via an env var `SNS_FORWARDER_ARN_PREFIX`

N.B make sure to add a trailing colon to the arn prefix e.g `--arn-prefix arn:aws:sns:us-east-1:123456789123:`

This allows the app to be used for forwarding alerts to multiple SNS topics. The rules for which alerts to send to which topics should be configured in Alertmanager.

### Deploying

In order to deploy the app on K8s the yaml file provided in folder `deploy` can be used. However, the deploy file requires some additional comments.

The app needs the AWS Region information in order to use the AWS SDK. This information is passed using an env variable. In the provided yaml file it's set to `eu-central-1`.

The app also needs AWS credentials to be able to use the AWS SDK to publish data to SNS. This can be achieved in a nice way using an IAM Role. The Role needs the following Policy attached:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "sns:Publish",
            "Resource": "<topic_arn>"
        }
    ]
}
```

If you're planning on publishing to multiple SNS topics, you should define them all as an array in the `Resource` field. Once the Role is ready, it should be attached to the container. This can be achieved on K8s using [kube2iam](https://github.com/jtblin/kube2iam) or [kiam](https://github.com/uswitch/kiam) via the `iam.amazonaws.com/role` annotation.

If for some reason this approach is not possbile, you should follow other [Best Practices for Managing AWS Access Keys](https://docs.aws.amazon.com/general/latest/gr/aws-access-keys-best-practices.html).


### Metrics

The app exposes [standard Go metrics](https://povilasv.me/prometheus-go-metrics/), [instrumented HTTP metrics](https://github.com/linki/instrumented_http) and the following custom metrics:

Name                                       | Description
-------------------------------------------|------------
`forwarder_sns_successful_requests_total`   | Total number of successful requests to SNS, with topic name as an additional label.
`forwarder_sns_unsuccessful_requests_total` | Total number of unsuccessful requests to SNS, with topic name as an additional label.

Additionally, the K8s deploy yaml file contains a definition of an appropriate Prometheus Service Monitor for scraping these metrics.
