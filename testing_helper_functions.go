package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
)

// deletes a single bucket specified by its bucket ID
// returns: bool indicating whether it was successfully deleted and response recorder
func deleteSingleBucket(bucketID string, username string) (bool, httptest.ResponseRecorder) {

	url := fmt.Sprintf("/api/v1/objects/%s", bucketID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Errorf(err.Error())
	}

	req.Header.Add("Authorization", "sage user:"+username)

	rr := httptest.NewRecorder()

	if status := rr.Code; status != http.StatusOK {
		log.Printf("response body: %s", rr.Body.String())
		log.Printf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
		return false, *rr
	}

	mainRouter.ServeHTTP(rr, req)
	log.Printf("Deleted: " + bucketID)
	return true, *rr
}

// deletes all buckets with IDs specified in the list
// returns: bool indicationg whether all buckets were successfully deleted
func deleteMultipleBuckets(bucketIDs []string, username string) (allBucketsDeleted bool) {
	allBucketsDeleted = true

	for _, bucketID := range bucketIDs {
		wasDeleted, _ := deleteSingleBucket(bucketID, username)
		if wasDeleted == false {
			allBucketsDeleted = false
		}
	}
	return
}

// returns all bucket IDs
func getAllBucketIDs(username string) (bucketIDs []string) {
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/v1/objects", nil)
	if err != nil {
		fmt.Errorf(err.Error())
	}
	req.Header.Add("Authorization", "sage user:"+username)
	mainRouter.ServeHTTP(rr, req)
	log.Printf(rr.Body.String())

	var buckets []SAGEBucket
	_ = json.Unmarshal([]byte(rr.Body.String()), &buckets)

	for _, bucket := range buckets {
		bucketIDs = append(bucketIDs, bucket.ID)
	}
	log.Printf("Bucket IDs: " + strings.Join(bucketIDs, ", "))
	return
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

// returns default values of username and datatype for testing and specified bucketName
func getNewTestingBucketSpecifications(bucketName string) (string, string, string) {
	return "testuser", "training-data", bucketName
}
