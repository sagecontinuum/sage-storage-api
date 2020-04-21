package main

import (
	"database/sql"
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

	mysqlHost     string
	mysqlDatabase string
	mysqlUsername string
	mysqlPassword string
	mysqlDSN      string // Data Source Name

	disableAuth = false // disable token introspection for testing purposes

	s3BucketPrefix = "sagedata-"
)

var validDataTypes = map[string]bool{
	"none":          true,
	"model":         true,
	"training-data": true,
	"profile":       true}

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

	mysqlHost = os.Getenv("MYSQL_HOST")
	mysqlDatabase = os.Getenv("MYSQL_DATABASE")
	mysqlUsername = os.Getenv("MYSQL_USER")
	mysqlPassword = os.Getenv("MYSQL_PASSWORD")

	// example: "root:password1@tcp(127.0.0.1:3306)/test"
	mysqlDSN = fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", mysqlUsername, mysqlPassword, mysqlHost, mysqlDatabase)

	log.Printf("mysqlHost: %s", mysqlHost)
	log.Printf("mysqlDatabase: %s", mysqlDatabase)
	log.Printf("mysqlUsername: %s", mysqlUsername)
	log.Printf("mysqlDSN: %s", mysqlDSN)
	count := 0
	for {
		count++
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			if count > 1000 {
				log.Fatalf("(sql.Open) Unable to connect to database: %v", err)
				return
			}
			log.Printf("(sql.Open) Unable to connect to database: %v, retrying...", err)
			time.Sleep(time.Second * 3)
			continue
		}
		//err = db.Ping()
		for {
			_, err = db.Exec("DO 1")
			if err != nil {
				if count > 1000 {
					log.Fatalf("(db.Ping) Unable to connect to database: %v", err)
					return
				}
				log.Printf("(db.Ping) Unable to connect to database: %v, retrying...", err)
				time.Sleep(time.Second * 3)
				continue
			}
			break
		}
		break
	}

	region := "us-west-2"
	disableSSL := false
	s3FPS := true
	maxMemory = 32 << 20 // 32Mb

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
	// api.Handle("/buckets", negroni.New(
	// 	negroni.HandlerFunc(authMW),
	// 	negroni.Wrap(http.HandlerFunc(listByBucket)),
	// )).Methods(http.MethodGet)
	// //Authenticated GET request:
	// //	get remote object from remote existing bucket
	// api.Handle("/buckets/{bucket}/{object}", negroni.New(
	// 	negroni.HandlerFunc(authMW),
	// 	negroni.Wrap(http.HandlerFunc(getObjectFromBucket)),
	// )).Methods(http.MethodGet)
	// //Authenticated POST Request:
	// //	post local object into remote existing bucket
	// api.Handle("/bucket", negroni.New(
	// 	negroni.HandlerFunc(authMW),
	// 	negroni.Wrap(http.HandlerFunc(putObjectInBucket)),
	// )).Methods(http.MethodPost)
	// -------------------------------------------------------

	// POST /objects/   create bucket
	api.Handle("/objects", negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(createBucketRequest)),
	)).Methods(http.MethodPost)

	// GET /objects/   list buckets
	api.Handle("/objects", negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(listSageBucketRequest)),
	)).Methods(http.MethodGet)

	// GET /objects/{bucket}/../   list bucket/folder contents OR download file
	api.NewRoute().PathPrefix("/objects/{bucket}").Handler(negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(getSageBucketGeneric)),
	)).Methods(http.MethodGet)

	// upload file
	// PUT /objects/{bucket}/{key...}
	api.NewRoute().PathPrefix("/objects/{bucket}/").Handler(negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(uploadObject)),
	)).Methods(http.MethodPut)

	// modify bucket
	// PATCH /objects/{bucket}
	api.NewRoute().Path("/objects/{bucket}").Handler(negroni.New(
		negroni.HandlerFunc(authMW),
		negroni.Wrap(http.HandlerFunc(patchBucket)),
	)).Methods(http.MethodPatch)

	api.NewRoute().PathPrefix("/").HandlerFunc(defaultHandler)

	log.Fatalln(http.ListenAndServe(":8080", api))

	// similar to S3 "Path-Style Request"

	// ****** buckets/folders ******
	// *** create bucket
	// POST /objects/ returns bucket id
	// *** list buckets
	// GET /objects/
	// *** list bucket/folder contents
	// GET /objects/{bucket}/{...}/ returns list

	// *** bucket properties
	// GET /objects/{bucket}/?metadata
	// GET /objects/{bucket}/?permission
	// PUT /objects/{bucket}/?metadata
	// PUT /objects/{bucket}/?permission

	// ****** files ******
	// *** upload file
	// PUT /objects/{id}/{key...} // PUT if bucket already exists, filename in key is optional
	// maybe: POST /objects/new/{key} // special case, bucket will be created

	// ** download file
	// GET /objects/{bucket}/{path...}/{filename}
}

// idea
//  GET /buckets/
//  GET /buckets/{bucket}

//  /files/{bucket}

// {bucket}.domain.com/path
