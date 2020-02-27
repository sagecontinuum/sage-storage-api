package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
)

var (
	endpoint        string
	accessKeyID     string
	secretAccessKey string
	useSSL          bool
	newSession      *session.Session
	svc             *s3.S3
	err             error
	filePath        string
)

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func init() {
	if len(os.Args) != 5 {
		exitErrorf("Endpoint, access key, secret key,and local file path"+
			"are required\nUsage: %s endPoint accessKey secretKey filePath",
			os.Args[0])
	}
	endpoint = os.Args[1]
	accessKeyID = os.Args[2]
	secretAccessKey = os.Args[3]
	filePath = os.Args[4]
	region := "us-west-2"
	disableSSL := false
	s3FPS := true

	// Initialize s3
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		DisableSSL:       aws.Bool(disableSSL),
		S3ForcePathStyle: aws.Bool(s3FPS),
	}
	newSession = session.New(s3Config)
	svc = s3.New(newSession)
}

func main() {
	r := mux.NewRouter()
	log.Println("Sage REST API")
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "api v1")
	})
	api.HandleFunc("/buckets", listByBucket).Methods(http.MethodGet)
	api.HandleFunc("/buckets/{bucket}/{object}", getObjectFromBucket).Methods(http.MethodGet)
	api.HandleFunc("/bucket", putObjectInBucket).Methods(http.MethodPost)
	log.Fatalln(http.ListenAndServe(":8080", r))
}
