package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v6"
)

var (
	endpoint        string
	accessKeyID     string
	secretAccessKey string
	useSSL          bool
	minioClient     *minio.Client
	err             error
	filePath        string
)

func init() {
	endpoint = os.Args[1]
	accessKeyID = os.Args[2]
	secretAccessKey = os.Args[3]
	filePath = os.Args[4]
	useSSL = true
	// Initialize minio client object.
	minioClient, err = minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("%#v\n", minioClient)
}

func main() {
	fmt.Println(os.Args[1:])
	r := mux.NewRouter()
	log.Println("Sage REST API")
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "api v1")
	})
	api.HandleFunc("/buckets", listByBucket).Methods(http.MethodGet)
	api.HandleFunc("/buckets/{bucket}/{object}", getObjectFromBucket).Methods(http.MethodGet)
	log.Fatalln(http.ListenAndServe(":8080", r))
}
