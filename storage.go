package main

import (
	"database/sql"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

// CreateS3Bucket ignore if already exists
func CreateS3Bucket(bucketName string) (err error) {
	_, err = svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Printf("bucket creation error: %s ", err.Error())
		// skip creation if it already exists
		if !strings.HasPrefix(err.Error(), s3.ErrCodeBucketAlreadyOwnedByYou) {
			err = nil
			fmt.Printf("Waiting for bucket %q to be created...\n", bucketName)

			err = svc.WaitUntilBucketExists(&s3.HeadBucketInput{
				Bucket: aws.String(bucketName),
			})

			if err != nil {
				err = fmt.Errorf("Unable to create bucket %q, %v", bucketName, err)
				return
			}

			log.Printf("bucket %s created", bucketName)
		}

	}
	return
}

func createSageBucket(username string, dataType string, bucketName string, isPublic bool) (sageBucket SAGEBucket, err error) {

	newUUID, err := uuid.NewRandom()
	if err != nil {
		err = fmt.Errorf("error generateing uuid %s", err.Error())
		return
	}

	bucketID := newUUID.String()

	log.Printf("using mysqlDSN: %s", mysqlDSN)

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	//resultArray := make([]string, 1)
	bucketCount := 0
	queryStr := "SELECT COUNT(*) FROM Buckets WHERE id=UUID_TO_BIN(?);"
	row := db.QueryRow(queryStr, &bucketID)

	err = row.Scan(&bucketCount)
	if err != nil {
		err = fmt.Errorf("Unable to query db: %v", err)
		return
	}

	log.Printf("buckets: %d", bucketCount)
	if bucketCount > 0 {
		// should never happen
		err = fmt.Errorf("SAGE bucket %s already exists", bucketID)
		return
	}

	s3BucketName := s3BucketPrefix + bucketID[0:2] // first two characters of uuid
	log.Printf("s3BucketName: %s", s3BucketName)
	err = CreateS3Bucket(s3BucketName)
	if err != nil {
		err = fmt.Errorf("Cannot create S3 bucket %s: %s", s3BucketName, err.Error())
		return
	}

	insertQueryStr := "INSERT INTO Buckets (id, name, owner, type) VALUES ( UUID_TO_BIN(?) , ?, ?, ?)  ;"
	_, err = db.Exec(insertQueryStr, bucketID, bucketName, username, dataType)
	if err != nil {
		err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
		return
	}

	// FULL_CONTROL
	insertQueryStr = "INSERT INTO BucketPermissions (id, granteeType, grantee, permission) VALUES ( UUID_TO_BIN(?) , ?, ?, ?) ON DUPLICATE KEY UPDATE permission=? ;"
	_, err = db.Exec(insertQueryStr, bucketID, "USER", username, "FULL_CONTROL", "FULL_CONTROL")
	if err != nil {
		err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
		return
	}

	// PUBLIC
	if isPublic {
		insertQueryStr = "INSERT INTO BucketPermissions (id, granteeType, grantee, permission) VALUES ( UUID_TO_BIN(?) , ?, ?, ?) ON DUPLICATE KEY UPDATE permission=? ;"
		_, err = db.Exec(insertQueryStr, bucketID, "GROUP", "AllUsers", "READ", "READ")
		if err != nil {
			err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
			return
		}
	}

	sageBucket, err = GetSageBucket(bucketID)
	if err != nil {
		err = fmt.Errorf("Bucket retrieval from mysql failed: %s", err.Error())
		return
	}
	//sageBucket = SAGEBucket{ID: bucketID, Name: bucketName, Owner: username, DataType: dataType}

	return
}

// userHasBucketPermission _
// check on any of 'READ', 'WRITE', 'READ_ACP', 'WRITE_ACP', 'FULL_CONTROL'
func userHasBucketPermission(granteeName string, bucketID string, requestPerm string) (ok bool, err error) {
	ok = false

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	granteeType := "USER"

	// TODO: infer group memberships

	matchCount := -1

	queryStr := ""
	var row *sql.Row

	queryStr = "SELECT COUNT(*) FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND ( granteeType=? AND grantee=? AND (permission='FULL_CONTROL' OR permission=? ));"

	log.Printf("queryStr: %s", queryStr)

	row = db.QueryRow(queryStr, bucketID, granteeType, granteeName, requestPerm)
	err = row.Scan(&matchCount)
	if err != nil {
		err = fmt.Errorf("db.QueryRow returned: %s (%s)", err.Error(), queryStr)
		return
	}
	if matchCount >= 1 {
		ok = true
	}
	return
}

// ListBucketPermissions _
func ListBucketPermissions(bucketID string) (permissions []*SAGEBucketPermission, err error) {

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	queryStr := "SELECT granteeType, grantee, permission FROM BucketPermissions WHERE id=UUID_TO_BIN(?) ;"

	log.Printf("ListBucketPermissions, queryStr: %s", queryStr)

	rows, err := db.Query(queryStr, bucketID)
	if err != nil {
		err = fmt.Errorf("db.Query returned: %s (%s)", err.Error(), queryStr)
		return
	}
	defer rows.Close()

	permissions = []*SAGEBucketPermission{}

	for rows.Next() {
		p := SAGEBucketPermission{}

		err = rows.Scan(&p.GranteeType, &p.Grantee, &p.Permission)
		if err != nil {
			err = fmt.Errorf("Could not parse row: %s", err.Error())
			return
		}
		//log.Printf("permission: %s", p.Permission)
		permissions = append(permissions, &p)
	}

	return
}

// listSageBuckets
// Note that the search does not search for bucket owners explictly, but for buckets for which user has FULL_CONTROL permission.
// TODO add pagination
func listSageBuckets(username string) (buckets []*SAGEBucket, err error) {

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	// *** get bucket ID's ***

	buckets = []*SAGEBucket{}

	// get list of bucket ID's for which user is owner OR bucket is public OR bucket is shared with user
	queryStr := "SELECT BIN_TO_UUID(id) FROM BucketPermissions WHERE (grantee=? AND permission='FULL_CONTROL') OR ((grantee=? OR grantee='group:AllUsers') AND permission='READ') ;"
	rows, err := db.Query(queryStr, username, username)
	if err != nil {
		err = fmt.Errorf("db.Query returned: %s (%s)", err.Error(), queryStr)
		return
	}
	defer rows.Close()

	var readBuckets []interface{}
	for rows.Next() {
		bucketID := ""
		err = rows.Scan(&bucketID)
		if err != nil {
			err = fmt.Errorf("Could not parse row: %s", err.Error())
			return
		}
		readBuckets = append(readBuckets, bucketID)
	}

	// *** get bucket objects ***

	// create variable-length (?, ?, ?)
	if len(readBuckets) == 0 {
		return
	}

	questionmarks := ""
	if len(readBuckets) == 1 {
		questionmarks = "?"
	} else {
		questionmarks = strings.Repeat("?, ", len(readBuckets)-1) + "?"
	}

	queryStr = fmt.Sprintf("SELECT BIN_TO_UUID(id), name, owner, type FROM Buckets WHERE BIN_TO_UUID(id) IN (%s);", questionmarks)
	rows, err = db.Query(queryStr, readBuckets...)
	if err != nil {
		err = fmt.Errorf("db.Query returned: %s (%s)", err.Error(), queryStr)
		return
	}
	defer rows.Close()

	for rows.Next() {
		b := new(SAGEBucket)
		err = rows.Scan(&b.ID, &b.Name, &b.Owner, &b.DataType)
		if err != nil {
			err = fmt.Errorf("Could not parse row: %s", err.Error())
			return
		}
		buckets = append(buckets, b)
	}

	return
}

// GetSageBucket _
func GetSageBucket(bucketID string) (s SAGEBucket, err error) {

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	queryStr := "SELECT BIN_TO_UUID(id), name, type, time_created, time_last_updated, owner FROM Buckets WHERE id=UUID_TO_BIN(?) ;"

	log.Printf("GetSageBucket, queryStr: %s", queryStr)

	row := db.QueryRow(queryStr, bucketID)

	s = SAGEBucket{}

	err = row.Scan(&s.ID, &s.Name, &s.DataType, &s.TimeCreated, &s.TimeUpdated, &s.Owner)
	if err != nil {
		err = fmt.Errorf("Could not parse row: %s", err.Error())
		return
	}

	return
}

func listSageBucketContent(sageBucketID string, folder string, user string) (files []string, err error) {

	s3BucketName := s3BucketPrefix + sageBucketID[0:2]

	log.Printf("s3BucketName: %s", s3BucketName)
	log.Printf("sageBucketID: %s", sageBucketID)

	loi := &s3.ListObjectsV2Input{
		Bucket:    aws.String(s3BucketName),
		Prefix:    aws.String(path.Join(sageBucketID, folder) + "/"),
		Delimiter: aws.String("/"),
	}

	log.Printf("loi: %s", loi.GoString())

	res, err := svc.ListObjectsV2(loi)
	if err != nil {
		err = fmt.Errorf("svc.ListObjectsV2 returned: %s", err.Error())
		return
	}
	files = []string{}

	log.Printf("found: %v", res)

	for _, object := range res.CommonPrefixes {
		s3key := *object.Prefix
		key := strings.TrimPrefix(s3key, sageBucketID+"/")
		//log.Printf("got: %s")
		files = append(files, key)
	}

	for _, object := range res.Contents {
		s3key := *object.Key
		key := strings.TrimPrefix(s3key, sageBucketID+"/")
		//log.Printf("got: %s")
		files = append(files, key)
	}

	return
}
