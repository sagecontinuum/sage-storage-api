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

	"github.com/google/uuid"
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

	upParams := &s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	}
	uploader := s3manager.NewUploader(newSession)
	_, err = uploader.Upload(upParams)
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", file, bucketName, err)
	}

	fmt.Fprintf(w, "Upload - Bucket: %v and Object: %v\n", bucketName, objectName)
	w.WriteHeader(http.StatusOK)
}

func uploadObject(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "body not parsed"}`))
		return
	}

	query := r.URL.Query()
	dataType := query.Get("type")
	if dataType == "" {
		http.Error(w, "Please specify data type via query field \"type\"", http.StatusInternalServerError)
		return
	}
	dataType = strings.ToLower(dataType)
	_, ok := validDataTypes[dataType]
	if !ok {
		http.Error(w, fmt.Sprintf("Data type %s not supported", dataType), http.StatusInternalServerError)
		return

	}

	newUUID, err := uuid.NewRandom()
	if err != nil {
		http.Error(w, "error generateing uuid", http.StatusInternalServerError)
		return

	}

	newUUIDStr := newUUID.String()
	objectName := newUUIDStr

	bucketName := dataType
	file, header, err := r.FormFile("file")
	//objectName := header.Filename
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	var objectMetadata map[string]*string
	objectMetadata = make(map[string]*string)

	filename := header.Filename
	log.Printf("filename: %s", filename)
	if filename != "" {
		objectMetadata["filename"] = &filename
	}
	upParams := &s3manager.UploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(objectName),
		Body:     file,
		Metadata: objectMetadata,
	}
	uploader := s3manager.NewUploader(newSession)
	_, err = uploader.Upload(upParams)
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", file, bucketName, err)
		return
	}

	log.Printf("Upload - Bucket: %v and Object: %v\n", bucketName, objectName)
	w.WriteHeader(http.StatusOK)
}

func mw(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	tokenStr := r.FormValue("token")
	url := tokenInfoEndpoint

	log.Printf("url: %s", url)

	payload := strings.NewReader("token=" + tokenStr)
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Fatal("NewRequest returned: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	auth := tokenInfoUser + ":" + tokenInfoPassword
	//fmt.Printf("auth: %s", auth)
	authEncoded := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Add("Authorization", "Basic "+authEncoded)

	req.Header.Add("Accept", "application/json; indent=4")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if res.StatusCode != 200 {
		fmt.Printf("%s", body)
		http.Error(w, fmt.Sprintf("token introspection failed (%d) (%s)", res.StatusCode, body), http.StatusInternalServerError)
		return
	}

	var dat map[string]interface{}
	if err := json.Unmarshal(body, &dat); err != nil {
		//fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	val, ok := dat["error"]
	if ok && val != nil {
		fmt.Fprintf(w, val.(string)+"\n")

		http.Error(w, val.(string), http.StatusInternalServerError)
		return

	}

	isActiveIf, ok := dat["active"]
	if !ok {
		http.Error(w, "field active was misssing", http.StatusInternalServerError)
		return
	}
	isActive, ok := isActiveIf.(bool)
	if !ok {
		http.Error(w, "field active is noty a boolean", http.StatusInternalServerError)
		return
	}

	if !isActive {
		http.Error(w, "token not active", http.StatusInternalServerError)
		return
	}

	next(w, r)

}
