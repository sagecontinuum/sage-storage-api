package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

var (
	//endpoint          string
	//accessKeyID       string
	//secretAccessKey   string
	tokenInfoEndpoint string
	tokenInfoUser     string
	tokenInfoPassword string

	useSSL     bool
	newSession *session.Session
	svc        *s3.S3
	err        error
	filePath   string
	maxMemory  int64

	disableAuth = false // disable token introspection for testing purposes
)

var validDataTypes = map[string]bool{
	"none":          true,
	"model":         true,
	"weights":       true,
	"training-data": true}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func init() {

	// token info
	flag.StringVar(&tokenInfoEndpoint, "tokenInfoEndpoint", "", "")
	flag.StringVar(&tokenInfoUser, "tokenInfoUser", "", "")
	flag.StringVar(&tokenInfoPassword, "tokenInfoPassword", "", "")

	// s3 endpoint
	var s3Endpoint string
	var s3accessKeyID string
	var s3secretAccessKey string

	flag.StringVar(&s3Endpoint, "s3Endpoint", "", "")
	flag.StringVar(&s3accessKeyID, "s3accessKeyID", "", "")
	flag.StringVar(&s3secretAccessKey, "s3secretAccessKey", "", "")

	flag.Parse()

	//if len(os.Args) != 6 {
	//	exitErrorf("Endpoint, access key, secret key, api server name and password "+
	//		"are required\nUsage: %s endPoint accessKey secretKey apiServer apiPassword",
	//		os.Args[0])
	//}

	if os.Getenv("TESTING_NOAUTH") == "1" {
		disableAuth = true
		log.Printf("WARNING: token validation is disabled, use only for testing/development")
		time.Sleep(time.Second * 2)
	}

	region := "us-west-2"
	disableSSL := false
	s3FPS := true
	maxMemory = 32 << 20

	// Initialize s3
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(s3accessKeyID, s3secretAccessKey, ""),
		Endpoint:         aws.String(s3Endpoint),
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
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(listByBucket)),
	)).Methods(http.MethodGet)
	//Authenticated GET request:
	//	get remote object from remote existing bucket
	api.Handle("/buckets/{bucket}/{object}", negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(getObjectFromBucket)),
	)).Methods(http.MethodGet)
	//Authenticated POST Request:
	//	post local object into remote existing bucket
	api.Handle("/bucket", negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(putObjectInBucket)),
	)).Methods(http.MethodPost)

	api.Handle("/objects/{bucket}/{key}", negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(downloadObject)),
	)).Methods(http.MethodGet)

	api.Handle("/objects/", negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(uploadObject)),
	)).Methods(http.MethodPost)

	log.Fatalln(http.ListenAndServe(":8080", api))

}
