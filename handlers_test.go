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
)

// TODO:
// test upload/download/delete of more than 1000 files to test correct handling of S3 pagination

func init() {
	if mainRouter == nil {
		go createRouter()
		time.Sleep(3 * time.Second) // TODO: this is not ideal yet...
	}

}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
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

// test of bucket deletion with more than 1000 files
func TestDeleteBigBucket(t *testing.T) {

	testuser := "testuser"
	dataType := "training-data"
	bucketName := "testing-bucket1"

	newBucket, err := createSageBucket(testuser, dataType, bucketName, false)
	if err != nil {
		t.Fatal(err)
	}

	bucketID := newBucket.ID

	fileCount := 1010

	for i := 0; i < fileCount; i++ {

		err = CreateFile(t, bucketID, testuser, fmt.Sprintf("mytestfile_%d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	filesInBucket, _, err := listSageBucketContent(bucketID, "/", false, int64(fileCount*2), "")
	if err != nil {
		t.Fatal(err)
	}

	//t.Logf("filesInBucket: %v", filesInBucket)

	if len(filesInBucket) != fileCount {
		t.Fatalf("expected %d files in bucket, only have %d", fileCount, len(filesInBucket))
	}

	url := fmt.Sprintf("/api/v1/objects/%s", bucketID)
	req, err := http.NewRequest("DELETE", url, nil)
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
	filesInBucket, _, err = listSageBucketContent(bucketID, "/", false, int64(fileCount*2), "")
	if err != nil {
		t.Fatal(err)
	}

	if len(filesInBucket) != 0 {
		t.Fatalf("expected 0 files in bucket (after deleting all), but have %d", len(filesInBucket))
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

	filesInBucket, _, err := listSageBucketContent(bucketID, "/", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}

	//t.Logf("filesInBucket: %v", filesInBucket)

	if len(filesInBucket) != 3 {
		t.Fatalf("expected 3 files in bucket, only have %d", len(filesInBucket))
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

	filesInBucket, _, err = listSageBucketContent(bucketID, "/", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(filesInBucket) != 2 {
		t.Fatalf("expected 2 files in bucket (after deleting one), only have %d", len(filesInBucket))
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
	fileArray := []string{}
	err = json.Unmarshal(rr.Body.Bytes(), &fileArray)
	if err != nil {
		t.Fatal(err)
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
	fileArray := []string{}
	err = json.Unmarshal(rr.Body.Bytes(), &fileArray)
	if err != nil {
		t.Fatal(err)
	}
	//fmt.Printf("fileArray: %v", fileArray)
	if len(fileArray) != 2 {
		t.Fatalf("error: expected two files %v", fileArray)
	}

	for _, file := range []string{"mytestfile1.txt", "directory/mytestfile2.txt"} {
		if !contains(fileArray, file) {
			t.Fatalf("error: did not find \"%s\"", file)
		}
	}

}
