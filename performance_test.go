package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	//"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestPerformance(t *testing.T) {
	/*
	   Optional test:
	   - multiple files are created with random content (e.g. from /dev/random ) and different sizes
	   - upload them in parallel at the same time
	   - afterwards download at the same time and the compare the files
	*/

	// define bucket specification
	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	// create SAGE bucket
	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	// create `file` struct that contains text that will be uploaded and downloaded from bucket
	type file struct {
		filename           string
		uploaded_content   string
		downloaded_content string
	}

	// create several `file` structs and put them in the list
	numberOfFiles := 15 // change number of files here
	min := 2000000      // change min number of characters in file here
	max := 6000000      // change max number of characters in file here
	fileList := []file{}
	newFile := file{}
	for index := 0; index < numberOfFiles; index++ {
		newFile = file{
			filename:           fmt.Sprintf("f%d.txt", index),
			uploaded_content:   createRandomString(rand.Intn(max-min+1) + min),
			downloaded_content: ""}
		fileList = append(fileList, newFile)
	}

	// upload files to the bucket in parallel
	var waitGroup sync.WaitGroup
	waitGroup.Add(numberOfFiles)
	for index := range fileList {
		go func(filename string, uploaded_content string) {
			defer waitGroup.Done()
			createFileWithContent(t, newBucket.ID, testuser, filename, uploaded_content)
		}(fileList[index].filename, fileList[index].uploaded_content)
	}
	waitGroup.Wait()

	// make bucket public
	body := ioutil.NopCloser(strings.NewReader(`{"granteeType": "GROUP", "grantee": "AllUsers", "permission": "READ"}`))
	url := fmt.Sprintf("/api/v1/objects/%s?permissions", newBucket.ID)
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)
	rr := httptest.NewRecorder()
	mainRouter.ServeHTTP(rr, req)

	// download files from the bucket in parallel
	var waitGroupDownload sync.WaitGroup
	waitGroupDownload.Add(numberOfFiles)
	for index := range fileList {
		go func(file *file) {
			defer waitGroupDownload.Done()
			*&file.downloaded_content = downloadFileFromBucket(t, newBucket.ID, file.filename)
		}(&fileList[index])
	}
	waitGroupDownload.Wait()

	// check if the content of files that were uploaded and downloaded from the bucket is the same
	for index := range fileList {
		if fileList[index].uploaded_content != fileList[index].downloaded_content {
			t.Fatalf("Uploaded and downloaded files don't match")
		}
	}
}

func createRandomString(string_length int) string {
	/*
		Create random string of length `string_length` compose of characters a-z, A-Z, and 0-9.
	*/

	// list character that the string may contain
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	// create array of required length
	s := make([]rune, string_length)

	// populate the array with random characters
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func downloadFileFromBucket(t *testing.T, bucketID string, filename string) (resultBody string) {
	/*
		Download file `filename` from bucket `bucketID`.
	*/

	// send GET request with bucket ID and filename to the router
	url := fmt.Sprintf("/api/v1/objects/%s/"+filename, bucketID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	mainRouter.ServeHTTP(rr, req)

	// check if we got a valid status code
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("Handler returned wrong status code: got %v want %v.", status, http.StatusOK)
	}

	// get content of file from the response struct
	resultBody = rr.Body.String()
	return resultBody
}

func createFileWithContentUploadRequest(t *testing.T, bucketID string, username string, key string, filename string, content string) (req *http.Request, err error) {
	/*
		Create file upload request that will create file `filename` in bucket `bucketID` with content `content`.
	*/

	// create file url
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	url := ""
	var part io.Writer
	if filename != "" {
		url = fmt.Sprintf("/api/v1/objects/%s/", bucketID)
		part, err = writer.CreateFormFile("file", filename)
	} else {
		url = fmt.Sprintf("/api/v1/objects/%s/%s", bucketID, key)
		part, err = writer.CreateFormField("file")
	}

	// set content of request
	part.Write([]byte(content))
	err = writer.Close()
	if err != nil {
		return
	}

	// create PUT request from URL
	t.Logf("PUT %s", url)
	req, err = http.NewRequest("PUT", url, body)
	if err != nil {
		return
	}

	// set autorization header for the request
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("Authorization", "sage user:"+username)
	return
}

func createFileWithContent(t *testing.T, bucketID string, username string, filename string, content string) (err error) {
	/*
		Create file `filename` in bucket `bucketID` and write string `content` into it.
	*/

	// create file upload request and send it to server
	req, err := createFileWithContentUploadRequest(t, bucketID, username, filename, "", content)
	if err != nil {
		return
	}
	rr := httptest.NewRecorder()
	mainRouter.ServeHTTP(rr, req)

	// check if file was created in the bucket.
	if status := rr.Code; status != http.StatusOK {
		err = fmt.Errorf("File not created.")
		return
	}
	return
}
