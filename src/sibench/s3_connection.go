package main

import "bytes"
import "fmt"
import "github.com/aws/aws-sdk-go/aws"
import "github.com/aws/aws-sdk-go/aws/credentials"
import "github.com/aws/aws-sdk-go/aws/session"
import "github.com/aws/aws-sdk-go/service/s3"
import "io"


type S3Connection struct {
    target string
    client *s3.S3
}


func CreateS3Connection(gateway string, port uint16, credentialMap map[string]string) (*S3Connection, error) {

    access_key := credentialMap["access_key"]
    secret_key := credentialMap["secret_key"]

    if access_key == "" {
        return nil, fmt.Errorf("Access key not provided in credentials")
    }

    if secret_key == "" {
        return nil, fmt.Errorf("Secret key not provided in credentials")
    }

    var creds = credentials.NewStaticCredentials(access_key, secret_key, "")
    var endpoint = fmt.Sprintf("%v:%v", gateway, port)
    var config = aws.NewConfig()

    config = config.WithRegion("us-east-1")
    config = config.WithDisableSSL(true)
	config = config.WithEndpoint(endpoint)
	config = config.WithS3ForcePathStyle(true)
	config = config.WithCredentials(creds)

    // Create an AWS session
    session, err := session.NewSession()
    if err != nil {
        return nil, err
    }

    fmt.Printf("Creating S3 Connection to %v\n", endpoint)

    var conn S3Connection
    conn.client = s3.New(session, config)
    conn.target = gateway

    return &conn, nil
}


func (conn *S3Connection) Target() string {
    return conn.target
}


func (conn *S3Connection) ListBuckets() ([]string, error) {
	result, err := conn.client.ListBuckets(nil)
    if err != nil {
        return nil, err
    }

    var buckets []string
	for _, b := range result.Buckets {
		buckets = append(buckets, aws.StringValue(b.Name))
	}

	return buckets, err
}


func (conn *S3Connection) CreateBucket(bucket string) error {
    fmt.Printf("Creating bucket on %v: %v\n", conn.target, bucket)
	_, err := conn.client.CreateBucket(&s3.CreateBucketInput{ Bucket: aws.String(bucket) })
	return err
}


func (conn *S3Connection) DeleteBucket(bucket string) error {
    fmt.Printf("Deleting bucket on %v: %v\n", conn.target, bucket)

    // We first have to delete the objects in the bucket.
    err := conn.DeleteObjects(bucket)
    if err != nil {
        return err
    }

	_, err = conn.client.DeleteBucket(&s3.DeleteBucketInput{ Bucket: aws.String(bucket) })
	return err
}


func (conn *S3Connection) DeleteObjects(bucket string) error {
    objKeys, err := conn.ListObjects(bucket)
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


func (conn *S3Connection) ListObjects(bucket string) ([]string, error) {
	result, err := conn.client.ListObjects(&s3.ListObjectsInput{ Bucket: aws.String(bucket) })

    var objects []string
    for _, o := range result.Contents {
        objects = append(objects, aws.StringValue(o.Key))
    }

	return objects, err
}


func (conn *S3Connection) PutObject(bucket string, key string, contents []byte) error {
    reader := bytes.NewReader(contents)

	_, err := conn.client.PutObject(&s3.PutObjectInput{
		Body:   reader,
		Bucket: &bucket,
		Key:    &key,
	})

	return err
}


func (conn *S3Connection) GetObject(bucket string, key string) ([]byte, error) {

	obj, err := conn.client.GetObject(&s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
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


func (conn *S3Connection) Close() {
    // Since S3 is a stateless protocol, there is no Close necessary.
}

