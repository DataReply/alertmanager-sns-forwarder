// Command alertmanager-sns-forwarder provides a Prometheus Alertmanager Webhook Receiver for forwarding alerts to AWS SNS.
package main

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/DataReply/alertmanager-sns-forwarder/arnutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/linki/instrumented_http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/gin-gonic/gin"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	log = logrus.New()

	listen_addr = kingpin.Flag("addr", "Address on which to listen").Default(":9087").Envar("SNS_FORWARDER_ADDRESS").String()
	debug       = kingpin.Flag("debug", "Debug mode").Default("false").Envar("SNS_FORWARDER_DEBUG").Bool()
	arnPrefix   = kingpin.Flag("arn-prefix", "Prefix to use for ARNs").Envar("SNS_FORWARDER_ARN_PREFIX").String()
	svc         *sns.SNS

	namespace = "forwarder"
	subsystem = "sns"
	labels    = []string{"topic"}

	snsRequestsSuccessful = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "successful_requests_total",
			Help:      "Total number of successful requests to SNS.",
		},
		labels,
	)

	snsRequestsUnsuccessful = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "unsuccessful_requests_total",
			Help:      "Total number of unsuccessful requests to SNS.",
		},
		labels,
	)
)

func main() {
	kingpin.Parse()

	registerCustomPrometheusMetrics()

	config := aws.NewConfig()

	config.WithHTTPClient(
		instrumented_http.NewClient(config.HTTPClient, &instrumented_http.Callbacks{
			PathProcessor: func(path string) string {
				parts := strings.Split(path, "/")
				return parts[len(parts)-1]
			},
		}),
	)

	session, err := session.NewSessionWithOptions(session.Options{
		Config: *config,
	})

	if err != nil {
		log.Error(err)
		return
	}

	if *arnPrefix == "" || !arnutil.ValidateARN(*arnPrefix) {
		log.Warn("ARN prefix not supplied or wrong, will try to detect")
		detectedArnPrefix, err := arnutil.DetectARNPrefix(session)
		if err != nil {
			log.Warnf("ARN prefix could not be detected, underlying problem is: %s. You will need to use the full topic ARN in the POST URL.", err)

		}
		arnPrefix = &detectedArnPrefix
	}

	svc = sns.New(session)

	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	} else {
		log.SetLevel(logrus.DebugLevel)
	}

	router := gin.Default()

	log.Info("listening on", *listen_addr)

	setupRouter(router)

	router.Run(*listen_addr)
}

func registerCustomPrometheusMetrics() {
	prometheus.MustRegister(snsRequestsSuccessful)
	prometheus.MustRegister(snsRequestsUnsuccessful)
}

// Helper function to set up Gin routes
func setupRouter(router *gin.Engine) {
	router.GET("/health", healthGETHandler)
	router.POST("/alert/:topic", alertPOSTHandler)
	router.GET("/metrics", prometheusHandler())
}

// Gin handler for Prometheus HTTP endpoint
func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func healthGETHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"health": "good",
	})
}

func alertPOSTHandler(c *gin.Context) {

	requestData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Error(err)
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}
	requestString := string(requestData)

	topic := c.Params.ByName("topic")
	topicArn := *arnPrefix + topic

	if !arnutil.ValidateARN(topicArn) {
		log.Errorf("The SNS topic ARN is not correct: %s", topicArn)
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Debugf("Using topic ARN: %s", topicArn)
	log.Debugln("+------------------  A L E R T  J S O N  -------------------+")
	log.Debugf("%s", requestString)
	log.Debugln("+-----------------------------------------------------------+")

	params := &sns.PublishInput{
		Message:  aws.String(requestString),
		TopicArn: aws.String(topicArn),
	}

	resp, err := svc.Publish(params)

	if err != nil {
		snsRequestsUnsuccessful.WithLabelValues(topic).Inc()
		log.Warn(err.Error())
		c.Writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	snsRequestsSuccessful.WithLabelValues(topic).Inc()
	log.Info(resp)
	c.Writer.WriteHeader(http.StatusOK)
}
