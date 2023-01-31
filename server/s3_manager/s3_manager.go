package s3_manager

import (
	"bytes"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/juju/errors"
)

type S3Manager struct {
	bucket  string
	region  string
	session *session.Session
}

type Opts struct {
	Bucket          string
	Region          string
	AccessKey       string
	SecretAccessKey string
}

func New(opts Opts) (*S3Manager, error) {
	session, err := session.NewSession(
		&aws.Config{
			Region: aws.String(opts.Region),
			Credentials: credentials.NewStaticCredentials(
				opts.AccessKey,
				opts.SecretAccessKey,
				"",
			),
		},
	)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &S3Manager{
		session: session,
		bucket:  opts.Bucket,
		region:  opts.Region,
	}, nil
}

func (s3m *S3Manager) UploadFile(from, to string) error {
	upFile, err := os.Open(from)
	if err != nil {
		return err
	}
	defer upFile.Close()

	upFileInfo, _ := upFile.Stat()
	var fileSize int64 = upFileInfo.Size()
	fileBuffer := make([]byte, fileSize)
	upFile.Read(fileBuffer)

	_, err = s3.New(
		s3m.session,
	).PutObject(
		&s3.PutObjectInput{
			Bucket:               aws.String(s3m.bucket),
			Key:                  aws.String(to),
			ACL:                  aws.String("private"),
			Body:                 bytes.NewReader(fileBuffer),
			ContentLength:        aws.Int64(fileSize),
			ContentType:          aws.String(http.DetectContentType(fileBuffer)),
			ContentDisposition:   aws.String("attachment"),
			ServerSideEncryption: aws.String("AES256"),
		},
	)
	return err
}
