package arnutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

var (
	regionString = "eu-central-1"

	testIAMData, _ = json.Marshal(ec2metadata.EC2IAMInfo{
		Code:               "Success",
		LastUpdated:        time.Now(),
		InstanceProfileArn: "arn:aws:iam::123456789012:instance-profile/profile",
		InstanceProfileID:  "123456789012",
	})

	mockUnavailableSession    = makeMockSession(http.StatusBadRequest, nil)()
	mockNoReturnedDataSession = makeMockSession(http.StatusOK, nil)()
	mockWorkingIAMSession     = makeMockSession(http.StatusOK, testIAMData)()
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

func TestValidateARN(t *testing.T) {

	if !ValidateARN("arn:aws:iam::123456789012:role/rolename") {
		t.Fatal("Correct ARN validated as wrong")
	}

	if ValidateARN(":aws:iam::123456789012:role/rolename") {
		t.Fatal("Wrong ARN validated as correct")
	}
}

func TestDetermineARNPrefix(t *testing.T) {

	// Test that function failes when Session is unavailable
	_, err := DetectARNPrefix(mockUnavailableSession)
	if err == nil {
		t.Fatal("Account prefix was created successfully but was supposed to fail")
	}

	// Test that function failes when Session is not working (not returning data)
	_, err = DetectARNPrefix(mockNoReturnedDataSession)
	if err == nil {
		t.Fatal("Account prefix was created successfully but was supposed to fail")
	}

	// Test that function failes when region is not provided
	_, err = DetectARNPrefix(mockWorkingIAMSession)
	if err == nil {
		t.Fatal("Account prefix was created successfully but was supposed to fail")
	}

	// Test that function succeeds when default region is provided
	os.Setenv("AWS_DEFAULT_REGION", "eu-central-1")
	_, err = DetectARNPrefix(mockWorkingIAMSession)
	if err != nil {
		t.Fatal("Account prefix was created unsuccessfully")
	}

	// Test that function succeeds when region is provided
	os.Setenv("AWS_REGION", "eu-central-1")
	_, err = DetectARNPrefix(mockWorkingIAMSession)
	if err != nil {
		t.Fatal("Account prefix was created unsuccessfully")
	}
}

func TestBaseSNSArn(t *testing.T) {

	// The ARN extracted by InstanceProfileArn() is a role
	instanceARN, err := arn.Parse("arn:aws:iam::123456789012:role/rolename")
	if err != nil {
		t.Fatal("Test instance ARN was parsed unsuccessfully")
	}

	// Test extracting the prefix from mock instance ARN is successful
	accountPrefix, err := BaseSNSArn(instanceARN, "eu-central-1")
	if err != nil {
		t.Fatal("Account prefix was created unsuccessfully")
	}

	// Test extracted prefix is valid
	_, err = arn.Parse(accountPrefix + ":123456789012")
	if err != nil {
		t.Fatal("Account prefix was created unsuccessfully")
	}
}

func TestGetRegionFromARN(t *testing.T) {

	region := GetRegionFromARN("arn:aws:iam::123456789012:role/rolename")

	if region != "" {
		t.Fatal("Region parsed from ARN without region was not empty")
	}

	region = GetRegionFromARN("arn:aws:iam:eu-central-1:123456789012:role/rolename")

	if region != "eu-central-1" {
		t.Fatal("Region parsed from ARN with region was not correct")
	}

	region = GetRegionFromARN(":aws:iam::123456789012:role/rolename")

	if region != "" {
		t.Fatal("Region parsed from wrong ARN was not empty")
	}
}
