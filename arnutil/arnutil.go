// Package arnutil provides utility functions for AWS ARNs.
package arnutil

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.New()
)

// Helper function to validate ARNs
func ValidateARN(arnString string) bool {
	_, err := arn.Parse(arnString)
	if err != nil {
		log.Warnf("The ARN supplied as argument does not parse successfully: %s", arnString)
		return false
	}
	return true
}

// Helper function to get region from ARNs
func GetRegionFromARN(arnString string) string {
	arn, err := arn.Parse(arnString)
	if err != nil {
		log.Warnf("The ARN supplied as argument does not parse successfully: %s", arnString)
		return ""
	}
	return arn.Region
}

// InstanceProfileArn uses the EC2 metadata API to find the role for
// the instance.
func InstanceProfileArn(svc *ec2metadata.EC2Metadata) (arn.ARN, error) {
	var retArn arn.ARN
	if !svc.Available() {
		return retArn, fmt.Errorf("aws metadata api not available")
	}

	info, err := svc.IAMInfo()
	if err != nil {
		return retArn, fmt.Errorf("error accessing iam info: %s", err)
	}

	return arn.Parse(info.InstanceProfileArn)
}

// BaseArn calculates the base SNS arn given an instance's arn
func BaseSNSArn(instanceProfileArn arn.ARN, region string) (string, error) {
	// instance profile arn will be of the form:
	// arn:aws:iam::account-id:instance-profile/role-name
	// so we extract account-id and compose an SNS ARN

	accountPrefix := strings.Join([]string{
		"arn",
		instanceProfileArn.Partition,
		endpoints.SnsServiceID,
		region,
		instanceProfileArn.AccountID}, ":")

	return fmt.Sprintf("%s:", accountPrefix), nil
}

// DetectARNPrefix uses the EC2 metadata API to determine the
// current prefix.
func DetectARNPrefix(sess *session.Session) (string, error) {

	svc := ec2metadata.New(sess)

	instanceArn, err := InstanceProfileArn(svc)
	if err != nil {
		return "", err
	}

	region, err := svc.Region()
	if region == "" || !ValidateRegionString(region) {
		// Could not get region, will try to get it from env
		if os.Getenv("AWS_REGION") != "" && ValidateRegionString(os.Getenv("AWS_REGION")) {
			region = os.Getenv("AWS_REGION")
		} else if os.Getenv("AWS_DEFAULT_REGION") != "" && ValidateRegionString(os.Getenv("AWS_DEFAULT_REGION")) {
			region = os.Getenv("AWS_DEFAULT_REGION")
		} else {
			return "", errors.New("The ARN prefix was not supplied and could not be detected.")
		}
	}

	return BaseSNSArn(instanceArn, region)
}

// Helper function to validate the region string
func ValidateRegionString(region string) bool {
	_, exists := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region)
	return exists
}
