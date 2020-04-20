package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gorilla/mux"

	"github.com/google/uuid"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

// SAGEBucket _
type SAGEBucket struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"` // optional name (might be indentical to only file in bucket) username/bucketname
	Owner       string            `json:"owner,omitempty"`
	DataType    string            `json:"type,omitempty"`
	TimeCreated *time.Time        `json:"time_created,omitempty"`
	TimeUpdated *time.Time        `json:"time_last_updated,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// SageFile simple response object
type SageFile struct {
	Bucket string `json:"bucket-id,omitempty"`
	Key    string `json:"key,omitempty"`
	Error  string `json:"error,omitempty"`
}

// SAGEBucketPermission _
type SAGEBucketPermission struct {
	User       string `json:"user,omitempty"`
	Permission string `json:"permission,omitempty"`
}

// ErrorStruct _
type ErrorStruct struct {
	Error string `json:"error"`
}

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

	insertQueryStr := "INSERT INTO Buckets (id, name, owner, type) VALUES ( UUID_TO_BIN(?) , ?, ?, ?) ;"
	_, err = db.Exec(insertQueryStr, bucketID, bucketName, username, dataType)
	if err != nil {
		err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
		return
	}

	// FULL_CONTROL
	insertQueryStr = "INSERT INTO BucketPermissions (id, user, permission) VALUES ( UUID_TO_BIN(?) , ?, ?) ;"
	_, err = db.Exec(insertQueryStr, bucketID, username, "FULL_CONTROL")
	if err != nil {
		err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
		return
	}

	// PUBLIC
	if isPublic {
		insertQueryStr = "INSERT INTO BucketPermissions (id, user, permission) VALUES ( UUID_TO_BIN(?) , ?, ?) ;"
		_, err = db.Exec(insertQueryStr, bucketID, "_public", "READ")
		if err != nil {
			err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
			return
		}
	}

	sageBucket = SAGEBucket{ID: bucketID, Name: bucketName, Owner: username, DataType: dataType}

	return
}

// userHasBucketPermission
// check on any of 'READ', 'WRITE', 'READ_ACP', 'WRITE_ACP', 'FULL_CONTROL'
func userHasBucketPermission(username string, bucketID string, requestPerm string) (ok bool, err error) {
	ok = false

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	matchCount := -1

	// WRITE implies READ
	// WRITE_ACP implies READ_ACP
	// FULL_CONTROL implies all
	// owner (not a permission) implies FULL_CONTROL
	// maybe? WRITE_ACP implies WRITE

	queryStr := ""
	var row *sql.Row
	switch requestPerm {
	case "WRITE":
		queryStr = "SELECT COUNT(*) FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND ( user=? AND (permission='FULL_CONTROL' OR permission='WRITE' ));"

	case "READ":
		// bucket is public OR user has either full, write or read permission
		queryStr = "SELECT COUNT(*) FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND ( ( user='_public' AND permission='READ' ) OR ( user=? AND (permission='FULL_CONTROL' OR permission='WRITE' OR permission='READ' )) );"

	case "READ_ACP":
		queryStr = "SELECT COUNT(*) FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND ( user=? AND (permission='FULL_CONTROL' OR permission='WRITE_ACP' OR permission='READ_ACP' )) ;"

	case "WRITE_ACP":
		queryStr = "SELECT COUNT(*) FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND ( user=? AND (permission='FULL_CONTROL' OR permission='WRITE_ACP' )) ;"

	default:
		err = fmt.Errorf("permission not supported")
		return
	}

	log.Printf("queryStr: %s", queryStr)

	row = db.QueryRow(queryStr, bucketID, username)
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

	queryStr := "SELECT user, permission FROM BucketPermissions WHERE id=UUID_TO_BIN(?) ;"

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

		err = rows.Scan(&p.User, &p.Permission)
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
	queryStr := "SELECT BIN_TO_UUID(id) FROM BucketPermissions WHERE (user=? AND permission='FULL_CONTROL') OR ((user=? OR user='_public') AND permission='READ') ;"
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

// getSageBucket
// return either a bucket/folder/ listing or a file
func getSageBucketGeneric(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	sageBucketID := vars["bucket"]
	if len(sageBucketID) != 36 {
		respondJSONError(w, http.StatusInternalServerError, "bucket id (%s) invalid (%d)", sageBucketID, len(sageBucketID))
	}

	username := vars["username"]

	// r.URL.Path example: /api/v1/objects/6dd46856-c871-4089-b1bc-a12b44e92c81

	//pathArray := strings.SplitN(r.URL.Path, "/", 6)

	//keyPath := pathArray[5]

	_, sagePath, err := getSagePath(r.URL.Path)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return

	}

	// bucket object
	if sagePath == "" {

		bucket, err := GetSageBucket(sageBucketID)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, bucket)
		return
	}

	// directory listing
	if strings.HasSuffix(sagePath, "/") {

		rawQuery := r.URL.RawQuery
		// this is ugle, .Values() does not handle ?permissions as it has no value
		log.Printf("rawQuery: %s", rawQuery)

		if strings.Contains(rawQuery, "permissions") {
			allowed, err := userHasBucketPermission(username, sageBucketID, "READ_ACP")
			if err != nil {
				respondJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			if !allowed {
				respondJSONError(w, http.StatusUnauthorized, "Access to bucket permissions denied (%s, %s)", username, sageBucketID)
				return
			}

			permissions, err := ListBucketPermissions(sageBucketID)
			if err != nil {
				respondJSONError(w, http.StatusBadRequest, err.Error())
				return
			}

			respondJSON(w, http.StatusOK, permissions)
			return

		}
		// _, sagePath, err := getSagePath(r.URL.Path)
		// if err != nil {
		// 	respondJSONError(w, http.StatusBadRequest, err.Error())
		// 	return

		// }

		files, err := listSageBucketContent(sageBucketID, sagePath, username)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "error listing bucket contents: %s", err.Error())
		}

		respondJSON(w, http.StatusOK, files)
		return
	}

	// a file download

	//uuidStr := vars["bucket"]
	//sageKey := vars["key"]

	// convert SAGE specifiers to S3 specifiers
	//sageBucketID := s3BucketPrefix + sageBucketID[0:2]

	s3BucketID := s3BucketPrefix + sageBucketID[0:2]

	s3key := path.Join(sageBucketID, sagePath)

	sageFilename := path.Base(sagePath)
	if sageFilename == "." || sageFilename == "/" {
		respondJSONError(w, http.StatusInternalServerError, "Invalid filename (%s)", sageFilename)
		return
	}

	objectInput := s3.GetObjectInput{
		Bucket: &s3BucketID,
		Key:    &s3key,
	}

	out, err := svc.GetObject(&objectInput)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Error getting data, svc.GetObject returned: %s", err.Error())
		return
	}
	defer out.Body.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+sageFilename)
	//w.Header().Set("Content-Length", FileSize)

	buffer := make([]byte, 1024*1024)
	w.WriteHeader(http.StatusOK)
	for {
		n, err := out.Body.Read(buffer)
		if err != nil {

			if err == io.EOF {
				w.Write(buffer[:n]) //should handle any remainding bytes.
				break
			}

			respondJSONError(w, http.StatusInternalServerError, "Error getting data: %s", err.Error())
			return
		}
		w.Write(buffer[0:n])
		//fmt.Println(string(p[:n]))
	}

	return
}

func listSageBucketRequest(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	username := vars["username"]

	buckets, err := listSageBuckets(username)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "error getting list of buckets: %s", err.Error())
	}

	respondJSON(w, http.StatusOK, buckets)
}

func getQueryField(r *http.Request, fieldName string) (value string, err error) {
	query := r.URL.Query()
	dataTypeArray, ok := query[fieldName]

	if !ok {
		err = fmt.Errorf("Please specify data type via query field \"type\"")
		return
	}

	if len(dataTypeArray) == 0 {
		err = fmt.Errorf("Please specify data type via query field \"type\"")
		return
	}

	value = dataTypeArray[0]
	if value == "" {
		err = fmt.Errorf("Please specify data type via query field \"type\"")
		return
	}

	value = strings.ToLower(value)
	_, ok = validDataTypes[value]
	if !ok {
		err = fmt.Errorf("Data type %s not supported", value)

		return

	}
	return
}

// POST /objects  creates SAGE bucket (and a S3 bucket if needed) and returns identifier
func createBucketRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("received bucket creation request")

	vars := mux.Vars(r)
	username := vars["username"]
	log.Printf("username: %s", username)

	dataType, err := getQueryField(r, "type")
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error(), dataType)
		return
	}

	bucketName, _ := getQueryField(r, "name")

	bucketObject, err := createSageBucket(username, dataType, bucketName, false)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "bucket creation failed: %s", err.Error())
		return
	}
	// TODO store owner info in mysql

	respondJSON(w, http.StatusOK, bucketObject)

}

func getSagePath(urlPath string) (bucket string, path string, err error) {
	pathParsed := strings.SplitN(urlPath, "/", 6)

	if len(pathParsed) != 6 && len(pathParsed) != 5 {
		err = fmt.Errorf("error parsing URL (%d)", len(pathParsed))
		return
	}

	if pathParsed[3] != "objects" {
		err = fmt.Errorf("error parsing URL (%s)", pathParsed[3])
		return
	}

	// bucket
	bucket = pathParsed[4]

	if len(pathParsed) == 6 {
		path = "/" + pathParsed[5] // folder or file
	} else {
		path = "" // bucket
	}
	log.Printf("getSagePath, path: %s", path)

	return
}

// PUT /objects/{bucket-id}/key... (woth or without filename in key)
func uploadObject(w http.ResponseWriter, r *http.Request) {

	pathParams := mux.Vars(r)
	sageBucketID := pathParams["bucket"]
	//objectKey := pathParams["key"]

	vars := mux.Vars(r)
	username := vars["username"]

	allowed, err := userHasBucketPermission(username, sageBucketID, "WRITE")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !allowed {
		respondJSONError(w, http.StatusUnauthorized, "Write access to bucket denied (%s, %s)", username, sageBucketID)
		return
	}

	log.Printf("r.URL.Path: %s", r.URL.Path)
	// example: /api/v1/objects/cbc2c709-2ef7-4852-8f5e-038fdc7f2304/test3/test3.jpg

	_, preliminarySageKey, err := getSagePath(r.URL.Path)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return

	}
	isDirectory := false
	if preliminarySageKey == "" || strings.HasSuffix(preliminarySageKey, "/") {
		isDirectory = true
	}

	log.Printf("preliminarySageKey: %s", preliminarySageKey)

	preliminaryS3Key := path.Join(sageBucketID, preliminarySageKey)
	log.Printf("preliminaryS3Key: %s", preliminaryS3Key)

	s3BucketName := s3BucketPrefix + sageBucketID[0:2] // first two characters of uuid
	log.Printf("s3BucketName: %s", s3BucketName)

	mReader, err := r.MultipartReader()
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "MultipartReader returned: %s", err.Error())
		return
	}

	//chunk := make([]byte, 4096)

	for {
		part, err := mReader.NextPart()
		if err != nil {
			if err != io.EOF {

				respondJSONError(w, http.StatusInternalServerError, "error reading part: %s", err.Error())
			} else {
				log.Printf("Hit last part of multipart upload")
				w.WriteHeader(200)
				fmt.Fprintf(w, "Upload completed")
			}
			return
		}

		// process part

		//formName := part.FormName()

		s3Key := ""

		sageKey := ""
		if isDirectory {
			filename := part.FileName()
			if filename == "" {
				respondJSONError(w, http.StatusBadRequest, "part upload has no filename")
				return
			}
			s3Key = path.Join(preliminaryS3Key, filename)
			sageKey = path.Join(preliminarySageKey, filename)

		} else {
			s3Key = preliminaryS3Key
			sageKey = preliminarySageKey
		}
		log.Printf("s3Key: %s", s3Key)
		log.Printf("sageKey: %s", sageKey)

		bufferedPartReader := bufio.NewReaderSize(part, 32768)

		var objectMetadata map[string]*string
		objectMetadata = make(map[string]*string)

		data := SageFile{}
		//data.Metadata = objectMetadata

		//key := filename // use filename unless key has been specified

		objectMetadata["owner"] = &username
		//objectMetadata["type"] = &dataType

		data.Key = sageKey

		upParams := &s3manager.UploadInput{
			Bucket:   aws.String(s3BucketName),
			Key:      aws.String(s3Key),
			Body:     bufferedPartReader,
			Metadata: objectMetadata,
		}
		uploader := s3manager.NewUploader(newSession)
		_, err = uploader.Upload(upParams)
		if err != nil {
			// Print the error and exit.
			respondJSONError(w, http.StatusInternalServerError, "Upload to S3 backend failed: %s", err.Error())
			return
		}

		data.Bucket = sageBucketID
		//log.Printf("Upload - Bucket: %v and Object: %v\n", bucketName, objectName)
		log.Printf("user upload successful")

		respondJSON(w, http.StatusOK, data)

		break // not doing multiple files yet
	}

	return
}

func downloadObject(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	uuidStr := pathParams["bucket"]
	sageKey := pathParams["key"]

	// convert SAGE specifiers to S3 specifiers
	sageBucketID := s3BucketPrefix + uuidStr[0:2]
	key := path.Join(uuidStr, sageKey)

	log.Printf("sageBucketID: %s key: %s", sageBucketID, key)

}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// json.NewEncoder(w).Encode(data)
	s, err := json.MarshalIndent(data, "", "  ")
	if err == nil {
		w.Write(s)
	}

}

func respondJSONError(w http.ResponseWriter, statusCode int, msg string, args ...interface{}) {
	errorStr := fmt.Sprintf(msg, args...)
	log.Printf("Reply to client: %s", errorStr)
	respondJSON(w, statusCode, ErrorStruct{Error: errorStr})
}

func authMW(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	vars := mux.Vars(r)

	if disableAuth {
		vars["username"] = "user-auth-disabled"
		next(w, r)
		return
	}

	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		respondJSONError(w, http.StatusInternalServerError, "Authorization header is missing")
		return
	}
	log.Printf("authorization: %s", authorization)
	authorizationArray := strings.Split(authorization, " ")
	if len(authorizationArray) != 2 {
		respondJSONError(w, http.StatusInternalServerError, "Authorization field must be of form \"sage <token>\"")
		return
	}

	if strings.ToLower(authorizationArray[0]) != "sage" {
		respondJSONError(w, http.StatusInternalServerError, "Only bearer \"sage\" supported")
		return
	}

	//tokenStr := r.FormValue("token")
	tokenStr := authorizationArray[1]
	log.Printf("tokenStr: %s", tokenStr)
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

	usernameIf, ok := dat["username"]
	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "username is missing")
		return
	}

	username, ok := usernameIf.(string)
	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "username is not string")
		return
	}

	//vars := mux.Vars(r)

	vars["username"] = username

	next(w, r)

}

func defaultHandler(w http.ResponseWriter, r *http.Request) {

	respondJSONError(w, http.StatusInternalServerError, "resource unknown")
	return
}
