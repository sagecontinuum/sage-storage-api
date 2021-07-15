package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

func init() {
	if mainRouter == nil {
		go createRouter()
		time.Sleep(3 * time.Second) // TODO: this is not ideal yet...
	}

}

func TestBucketCreation(t *testing.T) {

	req, err := http.NewRequest("POST", "/api/v1/objects?type=training-data&name=mybucket", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:testuser")

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	returnBucket := &SAGEBucket{}
	err = json.Unmarshal(rr.Body.Bytes(), &returnBucket)
	if err != nil {
		t.Fatal(err)
	}

	// example
	//{
	//	"id": "fda63c38-d27f-4a7c-affd-9c91fc65f3ac",
	//	"name": "mybucket",
	//	"type": "training-data"
	//}

	if len(returnBucket.ID) != 36 {
		t.Fatal("id wrong format")
	}

	if returnBucket.Name != "mybucket" {
		t.Fatal("name wrong")
	}

	if returnBucket.DataType != "training-data" {
		t.Fatal("type wrong")
	}

	if returnBucket.Owner != "testuser" {
		t.Fatalf("owner wrong, expected \"testuser\", got \"%s\"", returnBucket.Owner)
	}

}

func TestGetBucket(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	url := fmt.Sprintf("/api/v1/objects/%s", bucketID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	returnBucket := &SAGEBucket{}
	err = json.Unmarshal(rr.Body.Bytes(), &returnBucket)
	if err != nil {
		t.Fatal(err)
	}

	if len(returnBucket.ID) != 36 {
		t.Fatal("id wrong format")
	}

	if returnBucket.Name != bucketName {
		t.Fatal("name wrong")
	}

	if returnBucket.DataType != dataType {
		t.Fatal("type wrong")
	}

	if returnBucket.Owner != testuser {
		t.Fatalf("owner wrong, expected \"%s\", got \"%s\"", testuser, returnBucket.Owner)
	}

}

// curl -X PUT "localhost:8080/api/v1/objects/${BUCKET_ID}?permissions"
// -d '{"granteeType": "GROUP", "grantee": "AllUsers", "permission": "READ"}' -H "Authorization: sage ${SAGE_USER_TOKEN}"
func TestBucketPublic(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	// make bucket public

	body := ioutil.NopCloser(strings.NewReader(`{"granteeType": "GROUP", "grantee": "AllUsers", "permission": "READ"}`))

	url := fmt.Sprintf("/api/v1/objects/%s?permissions", bucketID)
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	returnObject := SAGEBucketPermission{}
	err = json.Unmarshal(rr.Body.Bytes(), &returnObject)
	if err != nil {
		t.Logf("return body: %s", rr.Body.String())
		t.Fatalf("json.Unmarshal returned: %s", err.Error())
	}

	if returnObject.Error != "" {
		t.Fatalf("returnObject.Error: \"%s\"", returnObject.Error)
	}

	if returnObject.Permission != "READ" {
		t.Fatalf("returned wrong permission, expected READ, got %s", returnObject.Permission)
	}

	// Try to view public bucket

	url = fmt.Sprintf("/api/v1/objects/%s", bucketID)
	req, err = http.NewRequest("GET", url, body)
	if err != nil {
		t.Fatal(err)
	}
	//req.Header.Add("Authorization", "sage user:"+testuser)

	rr = httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	return
}

//curl -X PATCH "localhost:8080/api/v1/objects/${BUCKET_ID}" -d '{"name":"new-bucket-name"}'  -H "Authorization: sage ${SAGE_USER_TOKEN}"
func TestRenameBucket(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	body := ioutil.NopCloser(strings.NewReader(`{"name":"new-bucket-name"}`))

	url := fmt.Sprintf("/api/v1/objects/%s", bucketID)
	req, err := http.NewRequest("PATCH", url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	returnObject := SAGEBucket{}
	err = json.Unmarshal(rr.Body.Bytes(), &returnObject)
	if err != nil {
		t.Logf("return body: %s", rr.Body.String())
		t.Fatalf("json.Unmarshal returned: %s", err.Error())
	}

	if returnObject.Error != "" {
		t.Fatalf("returnObject.Error: \"%s\"", returnObject.Error)
	}

	if returnObject.Name != "new-bucket-name" {
		t.Fatalf("returned wrong permission, expected \"new-bucket-name\", got %s", returnObject.Name)
	}

	return
}

// test of bucket deletion with more than 100 files (upload of 1000 is too slow)
func TestDeleteBigBucket(t *testing.T) {
	t.Logf("running TestDeleteBigBucket")

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "BUCKET_TO_BE_DELETED"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	fileCount := 100

	for i := 0; i < fileCount; i++ {

		err = CreateFile(t, bucketID, testuser, fmt.Sprintf("mytestfile_%d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	filesInBucketCount := 0
	ctoken := ""

	for true {

		listObject, err := listSageBucketContent(bucketID, "/", false, 10, "", ctoken)
		if err != nil {
			t.Fatal(err)
		}

		filesInBucketCount += len(listObject.Contents)
		if listObject.IsTruncated == nil || *listObject.IsTruncated == false {
			break
		}

		//if loop > 10 {
		//	t.Fatal(fmt.Errorf("loop did not end, ctoken: %s", ctoken))
		//}
		ctoken = *listObject.NextContinuationToken
	}

	//t.Logf("filesInBucket: %v", filesInBucket)

	if filesInBucketCount != fileCount {
		t.Fatalf("expected %d files in bucket, only have %d", fileCount, filesInBucketCount)
	}

	_, rr := deleteSingleBucket(bucketID, testuser)

	returnDeleteObject := &DeleteRespsonse{}
	err = json.Unmarshal(rr.Body.Bytes(), returnDeleteObject)
	if err != nil {
		t.Fatal(err)
	}

	if returnDeleteObject.Error != "" {
		t.Fatalf("error: %s", returnDeleteObject.Error)
	}

	if returnDeleteObject == nil {
		t.Fatal("returnDeleteObject == nil")
	}
	if len(returnDeleteObject.Deleted) != 1 {
		t.Fatalf("bucket has not been deleted (%d)", len(returnDeleteObject.Deleted))
	}

	// listSageBucketContent ignores the fact that bucket does not exist in mysql anymore
	filesInBucketCount = 0
	ctoken = ""
	for true {

		listObject, err := listSageBucketContent(bucketID, "/", false, 10, "", ctoken)
		if err != nil {
			t.Fatal(err)
		}

		filesInBucketCount += len(listObject.Contents)
		if listObject.IsTruncated == nil || *listObject.IsTruncated == false {
			break
		}

		//if loop > 10 {
		//	t.Fatal(fmt.Errorf("loop did not end, ctoken: %s", ctoken))
		//}
		ctoken = *listObject.NextContinuationToken
	}

	if filesInBucketCount != 0 {
		t.Fatalf("expected 0 files in bucket (after deleting all), but have %d", filesInBucketCount)
	}

}

// either user <key> or <filename>
func createFileUploadRequest(t *testing.T, bucketID string, username string, key string, filename string) (req *http.Request, err error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	url := ""
	var part io.Writer
	// TODO try CreateFormFile with filename
	if filename != "" {
		url = fmt.Sprintf("/api/v1/objects/%s/", bucketID)
		part, err = writer.CreateFormFile("file", filename)
	} else {
		url = fmt.Sprintf("/api/v1/objects/%s/%s", bucketID, key)
		part, err = writer.CreateFormField("file")
	}

	part.Write([]byte("test-data"))
	err = writer.Close()
	if err != nil {
		return
	}

	t.Logf("PUT %s", url)
	req, err = http.NewRequest("PUT", url, body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("Authorization", "sage user:"+username)
	return
}

func CreateFile(t *testing.T, bucketID string, username string, key string) (err error) {

	req, err := createFileUploadRequest(t, bucketID, username, key, "")
	if err != nil {
		return
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		err = fmt.Errorf("error creating file")
		return
	}

	return
}

// curl  -X PUT "localhost:8080/api/v1/objects/${BUCKET_ID}/{path}"  -H "Authorization: sage ${SAGE_USER_TOKEN}" -F 'file=@<filename>'
func TestFileUpload(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	req, err := createFileUploadRequest(t, bucketID, testuser, "", "testfile.txt")
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	//returnBucket := &SAGEBucket{}
	responseObject := SageFile{}

	err = json.Unmarshal(rr.Body.Bytes(), &responseObject)
	if err != nil {
		t.Fatal(err)
	}

	if responseObject.Error != "" {
		t.Fatalf("got error: %s", responseObject.Error)
	}

	if responseObject.Key != "testfile.txt" {
		t.Fatalf("filename wrong, got: \"%s\"", responseObject.Key)
	}

}

func TestFileUploadWithFilename(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	req, err := createFileUploadRequest(t, bucketID, testuser, "", "testfile2.txt")
	if err != nil {
		t.Fatal(err)
	}

	// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	//returnBucket := &SAGEBucket{}
	responseObject := SageFile{}

	err = json.Unmarshal(rr.Body.Bytes(), &responseObject)
	if err != nil {
		t.Fatal(err)
	}

	if responseObject.Error != "" {
		t.Fatalf("got error: %s", responseObject.Error)
	}

	if responseObject.Key != "testfile2.txt" {
		t.Fatalf("filename wrong, got: \"%s\"", responseObject.Key)
	}

}

func TestDeleteFile(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	// create three files
	err = CreateFile(t, bucketID, testuser, "mytestfile1.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = CreateFile(t, bucketID, testuser, "mytestfile2.txt")
	if err != nil {
		t.Fatal(err)
	}
	err = CreateFile(t, bucketID, testuser, "mytestfile3.txt")
	if err != nil {
		t.Fatal(err)
	}

	listObject, err := listSageBucketContent(bucketID, "/", false, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}

	//t.Logf("filesInBucket: %v", filesInBucket)

	if len(listObject.Contents) != 3 {
		t.Fatalf("expected 3 files in bucket, only have %d", len(listObject.Contents))
	}

	url := fmt.Sprintf("/api/v1/objects/%s/mytestfile1.txt", bucketID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	returnDeleteObject := &DeleteRespsonse{}
	err = json.Unmarshal(rr.Body.Bytes(), returnDeleteObject)
	if err != nil {
		t.Fatal(err)
	}

	if returnDeleteObject.Error != "" {
		t.Fatalf("error: %s", returnDeleteObject.Error)
	}

	if returnDeleteObject == nil {
		t.Fatal("returnDeleteObject == nil")
	}
	if len(returnDeleteObject.Deleted) != 1 {
		t.Fatalf("file has not beeen deleted (%d)", len(returnDeleteObject.Deleted))
	}

	listObject, err = listSageBucketContent(bucketID, "/", false, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(listObject.Contents) != 2 {
		t.Fatalf("expected 2 files in bucket (after deleting one), only have %d", len(listObject.Contents))
	}

}

func TestUploadAndDownload(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	// create three files
	err = CreateFile(t, bucketID, testuser, "mytestfile1.txt")
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("/api/v1/objects/%s/mytestfile1.txt", bucketID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	resultBody := rr.Body.String()

	if resultBody != "test-data" {
		t.Fatalf("file content wrongs, got: %s", resultBody)
	}

	// try download without permission
	url = fmt.Sprintf("/api/v1/objects/%s/mytestfile1.txt", bucketID)
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusUnauthorized)
	}

	// make bucket public
	body := ioutil.NopCloser(strings.NewReader(`{"granteeType": "GROUP", "grantee": "AllUsers", "permission": "READ"}`))

	url = fmt.Sprintf("/api/v1/objects/%s?permissions", bucketID)
	req, err = http.NewRequest("PUT", url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr = httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// test download as public user
	url = fmt.Sprintf("/api/v1/objects/%s/mytestfile1.txt", bucketID)
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	resultBody = rr.Body.String()

	if resultBody != "test-data" {
		t.Fatalf("file content wrongs, got: %s", resultBody)
	}

	// get a file listing

	url = fmt.Sprintf("/api/v1/objects/%s/", bucketID)
	req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	resultBody = rr.Body.String()

	if !strings.Contains(resultBody, "mytestfile1.txt") {
		t.Fatalf("file not found, got: %s", resultBody)
	}

}

func TestListFilesSimple(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	err = CreateFile(t, bucketID, testuser, "mytestfile1.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = CreateFile(t, bucketID, testuser, "directory/mytestfile2.txt")
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("/api/v1/objects/%s/", bucketID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		log.Printf("response body: %s", rr.Body.String())
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	log.Printf("response body: %s", rr.Body.String())
	listObject := s3.ListObjectsV2Output{}
	err = json.Unmarshal(rr.Body.Bytes(), &listObject)
	if err != nil {
		t.Fatal(err)
	}

	fileArray := []string{}
	for _, obj := range listObject.Contents {
		fileArray = append(fileArray, *obj.Key)
	}
	for _, obj := range listObject.CommonPrefixes {
		fileArray = append(fileArray, *obj.Prefix)
	}
	//fmt.Printf("fileArray: %v", fileArray)
	if len(fileArray) != 2 {
		t.Fatalf("error: expected two files %v", fileArray)
	}
	for _, file := range []string{"mytestfile1.txt", "directory/"} {
		if !contains(fileArray, file) {
			t.Fatalf("error: did not find \"%s\"", file)
		}
	}
	return
}

func TestListFilesRecursive(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	err = CreateFile(t, bucketID, testuser, "mytestfile1.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = CreateFile(t, bucketID, testuser, "directory/mytestfile2.txt")
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("/api/v1/objects/%s/?recursive", bucketID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Authorization", "sage user:"+testuser)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		log.Printf("response body: %s", rr.Body.String())
		t.Fatalf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
	log.Printf("response body: %s", rr.Body.String())
	//fileArray := []string{}
	listObject := s3.ListObjectsV2Output{}
	err = json.Unmarshal(rr.Body.Bytes(), &listObject)
	if err != nil {
		t.Fatal(err)
	}

	fileArray := []string{}
	for _, obj := range listObject.Contents {
		fileArray = append(fileArray, *obj.Key)
	}
	//t.Fatalf("got %v", listObject)
	//fmt.Printf("fileArray: %v", fileArray)
	if len(fileArray) != 2 {
		t.Fatalf("error: expected two files %v", fileArray)
	}

	for _, file := range []string{"mytestfile1.txt", "directory/mytestfile2.txt"} {
		if !contains(fileArray, file) {
			t.Fatalf("error: did not find \"%s\"", file)
		}
	}
	return
}

func TestListFilesMany(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	fileCount := 100

	for i := 1; i <= fileCount; i++ {
		err = CreateFile(t, bucketID, testuser, fmt.Sprintf("mytestfile_%d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	fileArray := []string{}

	cToken := ""
	url := fmt.Sprintf("/api/v1/objects/%s/", bucketID)
	for true {

		rr := httptest.NewRecorder()

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Authorization", "sage user:"+testuser)

		q := req.URL.Query()
		q.Add("limit", "10")

		if cToken != "" {
			q.Add("ContinuationToken", cToken)
		}

		req.URL.RawQuery = q.Encode()

		mainRouter.ServeHTTP(rr, req)

		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusOK {
			log.Printf("response body: %s", rr.Body.String())
			t.Fatalf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
		log.Printf("response body: %s", rr.Body.String())
		listObject := s3.ListObjectsV2Output{}
		err = json.Unmarshal(rr.Body.Bytes(), &listObject)
		if err != nil {
			log.Print("--------------")
			log.Print("body that could not be parsed: " + rr.Body.String())
			log.Print("--------------")
			t.Fatal(err)
		}

		for _, obj := range listObject.Contents {
			fileArray = append(fileArray, *obj.Key)
		}
		for _, obj := range listObject.CommonPrefixes {
			fileArray = append(fileArray, *obj.Prefix)
		}

		if !*listObject.IsTruncated {
			break
		}

		cToken = *listObject.NextContinuationToken
	}

	//fmt.Printf("fileArray: %v", fileArray)
	if len(fileArray) != fileCount {
		t.Fatalf("error: expected %d files, but got %d", fileCount, len(fileArray))
	}
	// for _, file := range []string{"mytestfile1.txt", "directory/"} {
	// 	if !contains(fileArray, file) {
	// 		t.Fatalf("error: did not find \"%s\"", file)
	// 	}
	// }
	return
}

// test for listSageBucketRequest function
func TestListSageBucketRequest(t *testing.T) {
	testuser, dataType, bucketName := getNewTestingBucketSpecifications("New_Bucket")

	// create several buckets and get their IDs
	var createdBucketIDs []string
	for i := 0; i < 10; i++ {
		sageBucket, err := createSageBucket(testuser, dataType, bucketName+fmt.Sprint(i), false)
		if err != nil {
			t.Fatalf("Problem with creating a bucket: " + err.Error())
		}
		createdBucketIDs = append(createdBucketIDs, sageBucket.ID)
	}

	// get IDs of all buckets
	allBucketIDs := getAllBucketIDs(testuser)

	// check if IDs of newly created buckets are in allBucketIDs array
	for createdBucketID := range createdBucketIDs {
		isInAllBucketsIDs := false
		for bucketID := range allBucketIDs {
			if createdBucketID == bucketID {
				isInAllBucketsIDs = true
			}
		}
		if isInAllBucketsIDs == false {
			t.Fatalf("ID of newly created bucket is not in all bucket IDs list.")
		}
	}
}

func TestPatchBucket(t *testing.T) {
	// create new bucket
	testuser, dataType, bucketName := getNewTestingBucketSpecifications("Patch_Bucket")

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	// change bucket's name
	url := fmt.Sprintf("/api/v1/objects/%s", newBucket.ID)
	jsonArg := []byte(`{"name": "Changed_Bucket_Name"}`)
	data := bytes.NewBuffer(jsonArg)

	req, err := http.NewRequest("PATCH", url, data)
	if err != nil {
		t.Errorf(err.Error())
	}

	req.Header.Add("Authorization", "sage user:"+testuser)
	rr := httptest.NewRecorder()
	mainRouter.ServeHTTP(rr, req)

	// check if we got a valid response
	if status := rr.Code; status != http.StatusOK {
		log.Printf("response body: %s", rr.Body.String())
		log.Printf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// check if we successfully changed bucket's name
	var changedBucket SAGEBucket
	_ = json.Unmarshal([]byte(rr.Body.String()), &changedBucket)
	if changedBucket.Name != "Changed_Bucket_Name" {
		log.Printf("Wrong bucket name: " + newBucket.Name)
		t.Error()
		return
	}
}
