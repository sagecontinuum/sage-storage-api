package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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
)

var validDataTypes = map[string]bool{
	"none":  true,
	"model": true}

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

	for dataType := range validDataTypes {
		bucket := dataType
		_, err = svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucket),
		})
		if err != nil {

			if strings.HasPrefix(err.Error(), s3.ErrCodeBucketAlreadyOwnedByYou) {
				// skip creation if it already exists
				continue
			}

			exitErrorf("Unable to create bucket %q, %v", bucket, err)
		}

		// Wait until bucket is created before finishing
		fmt.Printf("Waiting for bucket %q to be created...\n", bucket)

		err = svc.WaitUntilBucketExists(&s3.HeadBucketInput{
			Bucket: aws.String(bucket),
		})
	}
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

	api.Handle("/object", negroni.New(
		negroni.HandlerFunc(mw),
		negroni.Wrap(http.HandlerFunc(uploadObject)),
	)).Methods(http.MethodPost)

	log.Fatalln(http.ListenAndServe(":8080", api))

}
