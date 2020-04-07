package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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

	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &objectName,
	})
	defer out.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	Openfile, err := os.Create(objectName)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(Openfile, out.Body)
	if err != nil {
		log.Fatal(err)
	}
	FileStat, _ := Openfile.Stat()
	FileSize := strconv.FormatInt(FileStat.Size(), 10)
	w.Header().Set("Content-Disposition", "attachment; filename="+objectName)
	w.Header().Set("Content-Length", FileSize)
	http.ServeContent(w, r, Openfile.Name(), *out.LastModified, Openfile)
	fmt.Fprintf(w, "Download - Bucket: %v, Object: %v\n", bucketName, objectName)
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

func mw(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	tokenStr := r.FormValue("token")
	url := "https://sage.nautilus.optiputer.net/token_info/"
	payload := strings.NewReader("token=" + tokenStr)
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, payload)

	if err != nil {
		fmt.Println(err)
	}
	auth := apiServer + ":" + apiPassword
	authEncoded := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Add("Authorization", "Basic "+authEncoded)
	req.Header.Add("Accept", "application/json; indent=4")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var dat map[string]interface{}
	if err := json.Unmarshal(body, &dat); err != nil {
		fmt.Println(err)
	}
	val, ok := dat["error"]
	if ok && val != nil {
		fmt.Fprintf(w, val.(string)+"\n")
	} else {
		if dat["active"].(bool) {
			next(w, r)
		}
	}
}
