package main

import "bytes"
import "fmt"
import "github.com/aws/aws-sdk-go/aws"
import "github.com/aws/aws-sdk-go/aws/credentials"
import "github.com/aws/aws-sdk-go/aws/session"
import "github.com/aws/aws-sdk-go/service/s3"
import "logger"
import "io"


type S3Connection struct {
    gateway string
    protocol ProtocolConfig
    bucket string
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


func (conn *S3Connection) ManagerClose() error {
    err1 := conn.deleteBucket(conn.bucket)
    err2 := conn.WorkerClose()

    if err1 != nil {
        return err1
    }

    return err2
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


func (conn *S3Connection) WorkerClose() error {
    // Since S3 is a stateless protocol, there is no Close necessary.
    return nil
}


func (conn *S3Connection) createBucket(bucket string) error {
    logger.Infof("Creating bucket on %v: %v\n", conn.gateway, bucket)
	_, err := conn.client.CreateBucket(&s3.CreateBucketInput{ Bucket: aws.String(bucket) })
	return err
}


func (conn *S3Connection) deleteBucket(bucket string) error {
    logger.Infof("Deleting bucket on %v: %v\n", conn.gateway, bucket)

    // We first have to delete the objects in the bucket.
    err := conn.deleteObjects(bucket)
    if err != nil {
        return err
    }

	_, err = conn.client.DeleteBucket(&s3.DeleteBucketInput{ Bucket: aws.String(bucket) })
	return err
}


func (conn *S3Connection) listObjects(bucket string) ([]string, error) {
	result, err := conn.client.ListObjects(&s3.ListObjectsInput{ Bucket: aws.String(bucket) })

    var objects []string
    for _, o := range result.Contents {
        objects = append(objects, aws.StringValue(o.Key))
    }

	return objects, err
}


func (conn *S3Connection) deleteObjects(bucket string) error {
    objKeys, err := conn.listObjects(bucket)
    if err != nil {
        return  err
    }

	var objs = make([]*s3.ObjectIdentifier, len(objKeys))

	for i, key := range objKeys {
		objs[i] = &s3.ObjectIdentifier{Key: aws.String(key)}
	}

	var items s3.Delete
	items.SetObjects(objs)
	_, err = conn.client.DeleteObjects(&s3.DeleteObjectsInput{Bucket: &bucket, Delete: &items})

	return err
}


func (conn *S3Connection) PutObject(key string, id uint64, contents []byte) error {
    reader := bytes.NewReader(contents)

	_, err := conn.client.PutObject(&s3.PutObjectInput{
		Body:   reader,
		Bucket: &conn.bucket,
		Key:    &key,
	})

	return err
}


func (conn *S3Connection) GetObject(key string, id uint64) ([]byte, error) {

	obj, err := conn.client.GetObject(&s3.GetObjectInput{Bucket: aws.String(conn.bucket), Key: aws.String(key)})
    if err != nil {
        return nil, err
    }

    buf := bytes.NewBuffer(nil)
    _, err = io.Copy(buf, obj.Body)
    if err != nil {
	    return nil, err
	}

    return buf.Bytes(), nil
}


func (conn *S3Connection) InvalidateCache() error {
    return nil
}
