package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
)

func deleteSingleBucket(bucketID string, username string) {
	url := fmt.Sprintf("/api/v1/objects/%s", bucketID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Errorf(err.Error())
	}

	req.Header.Add("Authorization", "sage user:"+username)

	rr := httptest.NewRecorder()

	mainRouter.ServeHTTP(rr, req)
	log.Printf("Deleted: " + bucketID)
}

func deleteMultipleBuckets(bucketIDs []string, username string) {
	for _, bucketID := range bucketIDs {
		deleteSingleBucket(bucketID, username)
	}
}

func getAllBucketIDs(username string) (bucketIDs []string) {
	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/v1/objects", nil)
	if err != nil {
		fmt.Errorf(err.Error())
	}
	req.Header.Add("Authorization", "sage user:"+username)
	mainRouter.ServeHTTP(rr, req)

	var buckets []SAGEBucket
	_ = json.Unmarshal([]byte(rr.Body.String()), &buckets)

	for _, bucket := range buckets {
		bucketIDs = append(bucketIDs, bucket.ID)
	}
	log.Printf("Bucket IDs: " + strings.Join(bucketIDs, ", "))
	return
}
