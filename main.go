// Command alertmanager-sns-forwarder provides a Prometheus Alertmanager Webhook Receiver for forwarding alerts to AWS SNS.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"html/template"

	"github.com/DataReply/alertmanager-sns-forwarder/arnutil"
	"github.com/DataReply/alertmanager-sns-forwarder/template_util"
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

type Alerts struct {
	Alerts            []Alert                `json:"alerts"`
	CommonAnnotations map[string]interface{} `json:"commonAnnotations"`
	CommonLabels      map[string]interface{} `json:"commonLabels"`
	ExternalURL       string                 `json:"externalURL"`
	GroupKey          int                    `json:"groupKey"`
	GroupLabels       map[string]interface{} `json:"groupLabels"`
	Receiver          string                 `json:"receiver"`
	Status            string                 `json:"status"`
	Version           int                    `json:"version"`
}

type Alert struct {
	Annotations  map[string]interface{} `json:"annotations"`
	EndsAt       string                 `json:"endsAt"`
	GeneratorURL string                 `json:"generatorURL"`
	Labels       map[string]interface{} `json:"labels"`
	StartsAt     string                 `json:"startsAt"`
}

var (
	log = logrus.New()

	listen_addr           = kingpin.Flag("addr", "Address on which to listen").Default(":9087").Envar("SNS_FORWARDER_ADDRESS").String()
	debug                 = kingpin.Flag("debug", "Debug mode").Default("false").Envar("SNS_FORWARDER_DEBUG").Bool()
	arnPrefix             = kingpin.Flag("arn-prefix", "Prefix to use for ARNs").Envar("SNS_FORWARDER_ARN_PREFIX").String()
	templatePath          = kingpin.Flag("template-path", "Template path").Envar("SNS_FORWARDER_TEMPLATE_PATH").String()
	templateTimeZone      = kingpin.Flag("template-time-zone", "Template time zone").Envar("SNS_FORWARDER_TEMPLATE_TIME_ZONE").String()
	templateTimeOutFormat = kingpin.Flag("template-time-out-format", "Template time out format").Envar("SNS_FORWARDER_TEMPLATE_TIME_OUT_FORMAT").String()
	templateSplitToken    = kingpin.Flag("template-split-token", "Template split token").Envar("SNS_FORWARDER_TEMPLATE_SPLIT_TOKEN").String()
	svc                   *sns.SNS
	tmpH                  *template.Template

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

	// Template addictional functions map
	funcMap = template.FuncMap{
		"str_FormatDate":         template_util.Str_FormatDate,
		"str_UpperCase":          strings.ToUpper,
		"str_LowerCase":          strings.ToLower,
		"str_Title":              strings.Title,
		"str_FormatFloat":        template_util.Str_FormatFloat,
		"str_Format_Byte":        template_util.Str_Format_Byte,
		"str_Format_MeasureUnit": template_util.Str_Format_MeasureUnit,
		"HasKey":                 template_util.HasKey,
	}
)

func main() {
	kingpin.Parse()

	if templatePath != nil {
		tmpH = loadTemplate(templatePath)
	} else {
		tmpH = nil
	}

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

	router := gin.New()
	router.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/health", "/metrics"))
	router.Use(gin.Recovery())
	router.Run(*listen_addr)

	setupRouter(router)

	log.Info("listening on", *listen_addr)

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

func loadTemplate(tmplPath *string) *template.Template {
	// let's read template
	tmpH, err := template.New(path.Base(*tmplPath)).Funcs(funcMap).ParseFiles(*tmplPath)

	if err != nil {
		log.Fatalf("Problem reading parsing template file: %v", err)
	} else {
		log.Printf("Load template file:%s", *tmplPath)
	}

	return tmpH
}

func AlertFormatTemplate(alerts Alerts) string {
	var bytesBuff bytes.Buffer
	var err error

	writer := io.Writer(&bytesBuff)

	if *debug {
		log.Printf("Reloading Template\n")
		// reload template bacause we in debug mode
		tmpH = loadTemplate(templatePath)
	}

	tmpH.Funcs(funcMap)
	err = tmpH.Execute(writer, alerts)

	if err != nil {
		log.Fatalf("Problem with template execution: %v", err)
		panic(err)
	}

	return bytesBuff.String()
}

func alertPOSTHandler(c *gin.Context) {

	requestData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Error(err)
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}
	requestString := string(requestData)

	if templatePath != nil && tmpH != nil {
		var alerts Alerts

		json.Unmarshal(requestData, &alerts)

		requestString = AlertFormatTemplate(alerts)
	}

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
