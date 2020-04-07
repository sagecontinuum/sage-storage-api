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
	"github.com/urfave/negroni"
)

var (
	endpoint        string
	accessKeyID     string
	secretAccessKey string
	apiServer       string
	apiPassword     string
	useSSL          bool
	newSession      *session.Session
	svc             *s3.S3
	err             error
	filePath        string
	maxMemory       int64
)

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func init() {
	if len(os.Args) != 6 {
		exitErrorf("Endpoint, access key, secret key, api server name and password "+
			"are required\nUsage: %s endPoint accessKey secretKey apiServer apiPassword",
			os.Args[0])
	}
	endpoint = os.Args[1]
	accessKeyID = os.Args[2]
	secretAccessKey = os.Args[3]
	apiServer = os.Args[4]
	apiPassword = os.Args[5]
	region := "us-west-2"
	disableSSL := false
	s3FPS := true
	maxMemory = 32 << 20

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
		fmt.Fprintln(w, "Welcome to SAGE")
	})
	//Authenticated GET request:
	//	get the list of remote buckets
	api.Handle("/buckets", negroni.New(
		negroni.HandlerFunc(mw),
		negroni.Wrap(http.HandlerFunc(listByBucket)),
	)).Methods(http.MethodGet)
	//Authenticated GET request:
	//	get remote object from remote existing bucket
	api.Handle("/buckets/{bucket}/{object}", negroni.New(
		negroni.HandlerFunc(mw),
		negroni.Wrap(http.HandlerFunc(getObjectFromBucket)),
	)).Methods(http.MethodGet)
	//Authenticated POST Request:
	//	post local object into remote existing bucket
	api.Handle("/bucket", negroni.New(
		negroni.HandlerFunc(mw),
		negroni.Wrap(http.HandlerFunc(putObjectInBucket)),
	)).Methods(http.MethodPost)

	log.Fatalln(http.ListenAndServe(":8080", api))

}
