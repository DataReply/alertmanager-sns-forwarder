package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/gin-gonic/gin"
)

var (
	regionString = "eu-central-1"

	mockUnavailableSession    = makeMockSession(http.StatusBadRequest, nil)()
	mockNoReturnedDataSession = makeMockSession(http.StatusOK, nil)()

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
}

func TestPrometheusEndpoint(t *testing.T) {

	// Test that making requests to health endpoint results in OK status
	req, _ := http.NewRequest("GET", "/metrics", nil)
	testHTTPResponse(t, r, req, http.StatusOK)
}
