// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0

package main

import "bytes"
import "fmt"
import "github.com/aws/aws-sdk-go/aws"
import "github.com/aws/aws-sdk-go/aws/awserr"
import "github.com/aws/aws-sdk-go/aws/credentials"
import "github.com/aws/aws-sdk-go/aws/session"
import "github.com/aws/aws-sdk-go/service/s3"
import "io"
import "logger"


/*
 * A Connection for talking to S3 backend storage (or S3-like, such as Ceph + RadosGateway).
 */
type S3Connection struct {
    gateway string
    protocol ProtocolConfig
    bucket string
    bucketCreatedBySibench bool
    client *s3.S3
}


func NewS3Connection(target string, protocol ProtocolConfig, worker WorkerConnectionConfig) (*S3Connection, error) {
    var conn S3Connection
    conn.gateway = target
    conn.protocol = protocol
    conn.bucket = protocol["bucket"]
    return &conn, nil
}


func (conn *S3Connection) Target() string {
    return conn.gateway
}


func (conn *S3Connection) ManagerConnect() error {
    err := conn.WorkerConnect()
    if err != nil {
        return err
    }

    return conn.createBucket(conn.bucket)
}


func (conn *S3Connection) ManagerClose(cleanup bool) error {
    // Only delete the bucket if we created it
    if (cleanup && conn.bucketCreatedBySibench) {
        return conn.deleteBucket(conn.bucket)
    }

    return nil
}


func (conn *S3Connection) WorkerConnect() error {
    access_key := conn.protocol["access_key"]
    secret_key := conn.protocol["secret_key"]
    port := conn.protocol["port"]

    if access_key == "" {
        return fmt.Errorf("Access key not provided in protocol")
    }

    if secret_key == "" {
        return fmt.Errorf("Secret key not provided in protocol")
    }

    var creds = credentials.NewStaticCredentials(access_key, secret_key, "")
    var endpoint = fmt.Sprintf("%v:%v", conn.gateway, port)
    var awsConfig = aws.NewConfig()

    awsConfig = awsConfig.WithRegion("us-east-1")
    awsConfig = awsConfig.WithDisableSSL(true)
	awsConfig = awsConfig.WithEndpoint(endpoint)
	awsConfig = awsConfig.WithS3ForcePathStyle(true)
	awsConfig = awsConfig.WithCredentials(creds)

    // Create an AWS session
    session, err := session.NewSession()
    if err != nil {
        return err
    }

    logger.Infof("Creating S3 Connection to %v\n", endpoint)
    conn.client = s3.New(session, awsConfig)

    return nil
}


func (conn *S3Connection) WorkerClose(cleanup bool) error {
    // Since S3 is a stateless protocol, there is no Close necessary.
    return nil
}


func (conn *S3Connection) createBucket(bucket string) error {
    exists, err := conn.bucketExists(bucket)
    if exists {
        logger.Infof("Bucket already exists: %v\n", bucket)
        return nil
    }

    logger.Infof("Creating bucket on %v: %v\n", conn.gateway, bucket)
	_, err = conn.client.CreateBucket(&s3.CreateBucketInput{ Bucket: aws.String(bucket) })

    if err == nil {
        conn.bucketCreatedBySibench = true
    }

	return err
}


func (conn *S3Connection) deleteBucket(bucket string) error {
	_, err := conn.client.DeleteBucket(&s3.DeleteBucketInput{ Bucket: aws.String(bucket) })
	return err
}


/**
 * The S3 API says that creating a bucket if it already exists should fail with a
 * 'BucketAlreadyExists' error.  Unfortunately RGW does not implement the S3 protocol
 * correctly, so we will first need to test if it exists by using the HeadBucket call.
 */
func (conn *S3Connection) bucketExists(bucket string) (bool, error) {
	input := &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}

	_, err := conn.client.HeadBucket(input)
    if err == nil {
        return true, nil
    }

    // See if the particular error is the AWS error NoSuchBucket
    if aerr, ok := err.(awserr.Error); ok {
        if aerr.Code() == s3.ErrCodeNoSuchBucket {
            return false, nil
        }
    }

    return false, err
}


func (conn *S3Connection) RequiresKey() bool {
    return true
}

func (conn *S3Connection) CanDelete() bool {
    return true
}


func (conn *S3Connection) PutObject(key string, id uint64, buffer []byte) error {
    reader := bytes.NewReader(buffer)

	_, err := conn.client.PutObject(&s3.PutObjectInput{
		Body:   reader,
		Bucket: &conn.bucket,
		Key:    &key,
	})

	return err
}


func (conn *S3Connection) GetObject(key string, id uint64, buffer []byte) error {

    resp, err := conn.client.GetObject(&s3.GetObjectInput{Bucket: aws.String(conn.bucket), Key: aws.String(key)})
    if err != nil {
        return err
    }

    if *resp.ContentLength != int64(cap(buffer)) {
        return fmt.Errorf("Object has wrong size: expected %v, but got %v", cap(buffer), *resp.ContentLength)
    }

    pos := 0
	for true {
		n, err := resp.Body.Read(buffer[pos:])

        switch err {
            case nil:     pos += n
            case io.EOF:  return nil
            default:      return err
        }
    }

    return nil
}


func (conn *S3Connection) DeleteObject(key string, id uint64) error {

	_, err := conn.client.DeleteObject(&s3.DeleteObjectInput{
        Bucket: &conn.bucket,
        Key:    &key,
    })

    return err
}


func (conn *S3Connection) InvalidateCache() error {
    return nil
}
