package main

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/gin-gonic/gin"
)

var (
	regionString = "eu-central-1"

	data, err = ioutil.ReadFile("testdata/simple.json")

	mockUnavailableSession    = makeMockSession(http.StatusBadRequest, nil)()
	mockNoReturnedDataSession = makeMockSession(http.StatusOK, nil)()
	mockJsonDataSession       = makeMockSession(http.StatusOK, data)()

	r = gin.Default()
)

// This helper function makes a mock Session for passed status and optional data to return
func makeMockSession(status int, data []byte) func() *session.Session {
	return func() *session.Session {

		// server is the mock server that simply writes a status and optional data back to the client
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			if data != nil {
				w.Write(data)
			}
		}))

		return session.Must(session.NewSession(&aws.Config{
			DisableSSL:  aws.Bool(true),
			Endpoint:    aws.String(server.URL),
			Region:      &regionString,
			Credentials: credentials.NewStaticCredentials("AKID", "SECRET_KEY", "TOKEN"),
		}))
	}
}

// This function is used for setup before executing the test functions
func TestMain(m *testing.M) {
	//Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	registerCustomPrometheusMetrics()

	setupRouter(r)

	// Run the other tests
	os.Exit(m.Run())
}

// Helper function to process a request and test its response
func testHTTPResponse(t *testing.T, r *gin.Engine, req *http.Request, desiredStatus int) {

	// Create a response recorder
	w := httptest.NewRecorder()

	// Create the service and process the above request.
	r.ServeHTTP(w, req)

	if !(w.Code == desiredStatus) {
		t.Fail()
	}
}

type errReader int

func (errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("test error")
}

func TestHealthEndpoint(t *testing.T) {

	// Test that making requests to health endpoint results in OK status
	req, _ := http.NewRequest("GET", "/health", nil)
	testHTTPResponse(t, r, req, http.StatusOK)
}

func TestSNSAlertEndpoint(t *testing.T) {

	svc = sns.New(mockUnavailableSession)

	// Test that passing error payload results in BadRequest status
	req, _ := http.NewRequest("POST", "/alert/test-topic", errReader(0))
	testHTTPResponse(t, r, req, http.StatusBadRequest)

	// Test that passing wrong topic results in BadRequest status
	// Here simulated by wrong ARN prefix
	arnPrefixWrongTemp := "wrong"
	arnPrefix = &arnPrefixWrongTemp
	req, _ = http.NewRequest("POST", "/alert/test-topic", strings.NewReader("test-payload"))
	testHTTPResponse(t, r, req, http.StatusBadRequest)

	// Test that request using the unavailable mock Session results in ServiceUnavailable status
	arnPrefixCorrectTemp := "arn:aws:sns:eu-central-1:123456789012:"
	arnPrefix = &arnPrefixCorrectTemp
	req, _ = http.NewRequest("POST", "/alert/test-topic", strings.NewReader("test-payload"))
	testHTTPResponse(t, r, req, http.StatusServiceUnavailable)

	// Test that request using the available mock Session results in OK status
	svc = sns.New(mockNoReturnedDataSession)
	req, _ = http.NewRequest("POST", "/alert/test-topic", strings.NewReader("test-payload"))
	testHTTPResponse(t, r, req, http.StatusOK)

	templatePath_ := "testdata/default.tmpl"
	templatePath = &templatePath_
	tmpH = loadTemplate(templatePath)
	svc = sns.New(mockJsonDataSession)
	req, _ = http.NewRequest("POST", "/alert/test-topic", bytes.NewReader(data))
	testHTTPResponse(t, r, req, http.StatusOK)
}

func TestPrometheusEndpoint(t *testing.T) {

	// Test that making requests to health endpoint results in OK status
	req, _ := http.NewRequest("GET", "/metrics", nil)
	testHTTPResponse(t, r, req, http.StatusOK)
}

// Test_snsReturnCode helps ensure the correct HTTP return code is sent
// based on the type of SNS error returned.
func Test_snsReturnCode(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"No error", args{}, "2xx"},
		{"Non-AWS error", args{err: errors.New("Test error")}, "5xx"},
		{"Invalid Parameter", args{err: awserr.New(sns.ErrCodeInvalidParameterException, "", nil)}, "4xx"},
		{"Internal Error", args{err: awserr.New(sns.ErrCodeInternalErrorException, "", nil)}, "5xx"},
		{"Endpoint Disabled", args{err: awserr.New(sns.ErrCodeEndpointDisabledException, "", nil)}, "4xx"},
		{"Authorization Error", args{err: awserr.New(sns.ErrCodeAuthorizationErrorException, "", nil)}, "4xx"},
		{"KMS Disabled", args{err: awserr.New(sns.ErrCodeKMSDisabledException, "", nil)}, "4xx"},
		{"KMS Invalid State", args{err: awserr.New(sns.ErrCodeKMSInvalidStateException, "", nil)}, "4xx"},
		{"KMS Not Found", args{err: awserr.New(sns.ErrCodeKMSNotFoundException, "", nil)}, "4xx"},
		{"KMS Opt-in Reqd", args{err: awserr.New(sns.ErrCodeKMSOptInRequired, "", nil)}, "4xx"},
		{"KMS Throttle", args{err: awserr.New(sns.ErrCodeKMSThrottlingException, "", nil)}, "5xx"},
		{"KMS Access Denied", args{err: awserr.New(sns.ErrCodeKMSAccessDeniedException, "", nil)}, "4xx"},
		{"Invalid Security", args{err: awserr.New(sns.ErrCodeInvalidSecurityException, "", nil)}, "4xx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got := snsReturnCode(tt.args.err)
			var httpReturnCodeClass string

			switch {
			case got >= 500:
				httpReturnCodeClass = "5xx"
			case got >= 400:
				httpReturnCodeClass = "4xx"
			case got == 200:
				httpReturnCodeClass = "2xx"
			}

			if tt.want != httpReturnCodeClass {
				t.Errorf("snsReturnCode() = %v, want %v (%s)", got, tt.want, httpReturnCodeClass)
			}
		})
	}
}
