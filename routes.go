package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/minio/minio-go/v6"
)

func listByBucket(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	endpoint := ""
	accessKeyID := ""
	secretAccessKey := ""
	useSSL := true
	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("%#v\n", minioClient) // minioClient is now setup

	buckets, err := minioClient.ListBuckets()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, bucket := range buckets {
		fmt.Fprintf(w, "Bucket: %v\n", bucket)
	}
}
