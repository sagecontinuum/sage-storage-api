package main

import (
	"fmt"
	"io"
	"log"
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
	filePathDest := r.FormValue("filePathDest")

	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &objectName,
	})
	if err != nil {
		log.Fatal(err)
	}
	destFilePath := filePathDest + objectName
	destFile, err := os.Create(destFilePath)
	if err != nil {
		log.Fatal(err)
	}
	bytes, err := io.Copy(destFile, out.Body)
	if err != nil {
		log.Fatal(err)
	}
	out.Body.Close()
	destFile.Close()
	fmt.Fprintf(w, "Download - Bucket: %v, Object: %v, Size= %v bytes\n", bucketName, objectName, bytes)
	fmt.Fprintf(w, "Destination: %v\n", destFilePath)
	w.WriteHeader(http.StatusOK)
}

func putObjectInBucket(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "body not parsed"}`))
		return
	}
	bucketName := r.FormValue("bucket")
	file, header, err := r.FormFile("file")
	objectName := header.Filename
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
	w.WriteHeader(http.StatusOK)
}
