package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
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
	log.Printf(bucketID + " deleted")
}

func deleteMultipleBuckets(bucketIDs []string, username string) {
	for _, bucketID := range bucketIDs {
		deleteSingleBucket(bucketID, username)
	}
}
