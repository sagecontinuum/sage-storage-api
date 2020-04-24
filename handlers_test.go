package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
