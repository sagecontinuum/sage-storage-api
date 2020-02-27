package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gorilla/mux"
)

func listByBucket(w http.ResponseWriter, r *http.Request) {
	result, err := svc.ListBuckets(nil)
	if err != nil {
		exitErrorf("Unable to list buckets, %v", err)
	}

	fmt.Fprintf(w, "Buckets:\n")

	for _, b := range result.Buckets {
		fmt.Fprintf(w, "* %s created on %s\n",
			aws.StringValue(b.Name), aws.TimeValue(b.CreationDate))
	}
	w.WriteHeader(http.StatusOK)
}

func getObjectFromBucket(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	bucketName := pathParams["bucket"]
	objectName := pathParams["object"]
	localFile, err := os.Create(filePath + objectName)
	if err != nil {
		fmt.Println("Failed to create file", err)
		return
	}
	defer localFile.Close()
	downloader := s3manager.NewDownloader(newSession)
	numBytes, err := downloader.Download(localFile,
		&s3.GetObjectInput{Bucket: &bucketName, Key: &objectName})
	if err != nil {
		fmt.Println("Failed to download file.", err)
		return
	}
	fmt.Fprintf(w, "Download - Bucket: %v, Object: %v, Size= %v bytes\n", bucketName, objectName, numBytes)
	fmt.Fprintf(w, "Destination: %v\n", filePath+objectName)
	w.WriteHeader(http.StatusOK)
}

func putObjectInBucket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "body not parsed"}`))
		return
	}
	bucketName := r.FormValue("bucket")
	objectName := r.FormValue("object")

	file, err := os.Open(filePath + objectName)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	uploader := s3manager.NewUploader(newSession)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	})
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", file, bucketName, err)
	}

	fmt.Fprintf(w, "Upload - Bucket: %v and Object: %v\n", bucketName, objectName)
	fmt.Fprintf(w, "Location: %v\n", filePath+objectName)
	w.WriteHeader(http.StatusOK)
}
