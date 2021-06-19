package aws

import (
	"bytes"
	"fmt"

	"github.com/SevenTV/ServerGo/src/configure"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	log "github.com/sirupsen/logrus"
)

var sess = session.Must(session.NewSession(&aws.Config{
	Credentials: credentials.NewStaticCredentials(configure.Config.GetString("aws_akid"), configure.Config.GetString("aws_secret_key"), configure.Config.GetString("aws_session_token")),
	Region:      aws.String(configure.Config.GetString("aws_region")),
	Endpoint:    aws.String(configure.Config.GetString("aws_endpoint")),
}))

var svc = s3.New(sess)

var uploader = s3manager.NewUploader(sess)

func UploadFile(bucket, key string, body []byte, contentType *string) error {
	// The session the S3 Uploader will use

	// Create an uploader with the session and default options
	// Upload the file to S3.
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(body),
		ACL:          aws.String("public-read"),
		ContentType:  contentType,
		CacheControl: aws.String("public, max-age=15552000"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	log.Debugf("file uploaded to, %s", result.Location)
	return nil
}

func Expire(bucket, key string, number int) error {
	obj := fmt.Sprintf("deleted/%s/%vx", key, number)

	sourceObject := fmt.Sprintf("%s/%s/%vx", bucket, key, number)
	_, err := svc.CopyObject(&s3.CopyObjectInput{
		ACL:        aws.String("private"),
		Bucket:     aws.String(bucket),
		CopySource: aws.String(sourceObject),
		Key:        aws.String(obj),
	})

	if err != nil {
		return fmt.Errorf("unable to expire object %q from bucket %q, %v", key, bucket, err)
	}

	err = svc.WaitUntilObjectExists(&s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(obj)})
	if err != nil {
		return fmt.Errorf("unable to expire object %q from bucket %q, %v", key, bucket, err)
	}

	return DeleteFile(bucket, fmt.Sprintf("%s/%vx", key, number), false)
}

func Unexpire(bucket, key string, number int) error {
	obj := fmt.Sprintf("%s/%vx", key, number)

	sourceObject := fmt.Sprintf("%s/deleted/%s/%vx", bucket, key, number)
	_, err := svc.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(sourceObject),
		Key:        aws.String(obj),
	})

	if err != nil {
		return fmt.Errorf("unable to expire object %q from bucket %q, %v", key, bucket, err)
	}

	err = svc.WaitUntilObjectExists(&s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(obj)})
	if err != nil {
		return fmt.Errorf("unable to expire object %q from bucket %q, %v", key, bucket, err)
	}

	return DeleteFile(bucket, fmt.Sprintf("deleted/%s/%vx", key, number), false)
}

func DeleteFile(bucket, key string, wait bool) error {
	_, err := svc.DeleteObject(&s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("unable to delete object %q from bucket %q, %v", key, bucket, err)
	}
	if wait {
		return svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	}
	return nil
}
