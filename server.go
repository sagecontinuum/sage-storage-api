package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
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
	mw              *jwtmiddleware.JWTMiddleware
	maxMemory       int64
)

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func init() {
	if len(os.Args) != 4 {
		exitErrorf("Endpoint, access key, and secret key "+
			"are required\nUsage: %s endPoint accessKey secretKey",
			os.Args[0])
	}
	endpoint = os.Args[1]
	accessKeyID = os.Args[2]
	secretAccessKey = os.Args[3]
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

	mw = jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(secretAccessKey), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
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
		negroni.HandlerFunc(mw.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(listByBucket)),
	)).Methods(http.MethodGet)
	//Authenticated GET request:
	//	get remote object from remote existing bucket
	api.Handle("/buckets/{bucket}/{object}", negroni.New(
		negroni.HandlerFunc(mw.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(getObjectFromBucket)),
	)).Methods(http.MethodGet)
	//Authenticated POST Request:
	//	post local object into remote existing bucket
	api.Handle("/bucket", negroni.New(
		negroni.HandlerFunc(mw.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(putObjectInBucket)),
	)).Methods(http.MethodPost)

	log.Fatalln(http.ListenAndServe(":8080", api))

}
