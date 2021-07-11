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
	"strconv"
	"strings"
	"time"

	// "bytes"
	// "mime/multipart"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gorilla/mux"

	"database/sql"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
)

// SAGEBucket _
type SAGEBucket struct {
	ErrorStruct `json:",inline"`
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"` // optional name (might be indentical to only file in bucket) username/bucketname
	Owner       string            `json:"owner,omitempty"`
	DataType    string            `json:"type,omitempty"`
	TimeCreated *time.Time        `json:"time_created,omitempty"`
	TimeUpdated *time.Time        `json:"time_last_updated,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SageFile simple response object
type SageFile struct {
	ErrorStruct `json:",inline"`
	Bucket      string `json:"bucket-id,omitempty"`
	Key         string `json:"key,omitempty"`
}

// SAGEBucketPermission _
type SAGEBucketPermission struct {
	ErrorStruct `json:",inline"`
	GranteeType string `json:"granteeType,omitempty"`
	Grantee     string `json:"grantee,omitempty"`
	Permission  string `json:"permission,omitempty"`
}

// DeleteRespsonse _
type DeleteRespsonse struct {
	ErrorStruct `json:",inline"`
	Deleted     []string `json:"deleted"`
}

// ErrorStruct _
type ErrorStruct struct {
	Error string `json:"error,omitempty"`
}

// getSageBucket
// return either a bucket/folder/ listing or a file
func getSageBucketGeneric(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	sageBucketID := vars["bucket"]
	if len(sageBucketID) != 36 {
		respondJSONError(w, http.StatusInternalServerError, "bucket id (%s) invalid (%d)", sageBucketID, len(sageBucketID))
		return
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

	// bucket permission
	rawQuery := strings.ToLower(r.URL.RawQuery) // this is ugle, .Values() does not handle ?permissions as it has no value

	//log.Printf("rawQuery: %s", rawQuery)
	if sagePath == "" && strings.Contains(rawQuery, "permissions") {

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

	// bucket of directory listing

	allowed, err := userHasBucketPermission(username, sageBucketID, "READ")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !allowed {
		respondJSONError(w, http.StatusUnauthorized, "Read access to bucket denied (username: \"%s\", sageBucketID: \"%s\")", username, sageBucketID)
		return
	}

	if sagePath == "" {
		// bucket listing
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

		// _, sagePath, err := getSagePath(r.URL.Path)
		// if err != nil {
		// 	respondJSONError(w, http.StatusBadRequest, err.Error())
		// 	return

		// }

		recursive := false
		if strings.Contains(rawQuery, "recursive") {
			if !strings.Contains(rawQuery, "recursive=false") {
				recursive = true
			}

		}
		continuationToken, err := getQueryField(r, "ContinuationToken")
		if err != nil {
			continuationToken = ""
		}

		limit, err := getQueryFieldInt64(r, "limit", 0)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "error parsing query field limit: %s", err.Error())
			return
		}

		if false {
			files, directories, newContinuationToken, err := listSageBucketContentDepreccated(sageBucketID, sagePath, recursive, 0, "", continuationToken)

			if err != nil {
				respondJSONError(w, http.StatusInternalServerError, "error listing bucket contents (sageBucketID: %s, sagePath: %s): %s", sageBucketID, sagePath, err.Error())
				return
			}
			files = append(files, directories...)

			// Repsonse has some similiarity to https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListObjectsV2.html#API_ListObjectsV2_ResponseSyntax

			data := make(map[string]interface{})
			data["Contents"] = files

			if newContinuationToken != "" {
				data["NextContinuationToken"] = newContinuationToken
				data["IsTruncated"] = true
			} else {
				data["IsTruncated"] = false
			}

			respondJSON(w, http.StatusOK, data)
		}

		listObject, err := listSageBucketContent(sageBucketID, sagePath, recursive, limit, "", continuationToken)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "error listing bucket contents (sageBucketID: %s, sagePath: %s): %s", sageBucketID, sagePath, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, listObject)

		return
	}

	// a file download

	//uuidStr := vars["bucket"]
	//sageKey := vars["key"]

	// convert SAGE specifiers to S3 specifiers
	//sageBucketID := s3BucketPrefix + sageBucketID[0:2]

	s3BucketID := getS3BucketID(sageBucketID) //s3BucketPrefix + sageBucketID[0:2]

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
				fileDownloadByteSize.Add(float64(n))
				break
			}

			respondJSONError(w, http.StatusInternalServerError, "Error getting data: %s", err.Error())
			return
		}
		w.Write(buffer[0:n])
		fileDownloadByteSize.Add(float64(n))
	}

	return
}

func listSageBucketRequest(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	username := vars["username"]

	filter_owner, _ := getQueryField(r, "owner")

	filter_name, _ := getQueryField(r, "name")
	//if err != nil {
	//	respondJSONError(w, http.StatusInternalServerError, "error parsiong query: %s", err.Error())
	//	return
	//}

	buckets, err := listSageBuckets(username, filter_owner, filter_name)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "error getting list of buckets: %s", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, buckets)
}

func getQueryFieldBool(r *http.Request, fieldName string) (value bool, err error) {

	value = false

	valueStr, err := getQueryField(r, fieldName)
	if err != nil {
		return
	}

	if valueStr == "true" || valueStr == "1" {
		value = true
	}

	return
}

func getQueryFieldInt64(r *http.Request, fieldName string, defaultValue int64) (value int64, err error) {

	query := r.URL.Query()
	_, ok := query[fieldName]

	if !ok {
		value = defaultValue
		return
	}

	valueStr, err := getQueryField(r, fieldName)
	if err != nil {
		return
	}

	value, err = strconv.ParseInt(valueStr, 10, 64)
	//value, err = strconv.Atoi(valueStr)
	if err != nil {
		return
	}

	return
}

// only gets the fist value, even if there are multiple values
func getQueryField(r *http.Request, fieldName string) (value string, err error) {
	query := r.URL.Query()
	dataTypeArray, ok := query[fieldName]

	if !ok {
		err = fmt.Errorf("Field %s not found", fieldName)
		return
	}

	if len(dataTypeArray) == 0 {
		err = fmt.Errorf("Field %s not found", fieldName)
		return
	}

	value = dataTypeArray[0]
	if value == "" {
		err = fmt.Errorf("Field %s is empty", fieldName)
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

	_, ok := validDataTypes[dataType]
	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "Data type %s not supported", dataType)
		return

	}

	bucketName, _ := getQueryField(r, "name")

	isPublic, _ := getQueryFieldBool(r, "public")

	bucketObject, err := createSageBucket(username, dataType, bucketName, isPublic)
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

// patchBucket  Modify bucket metadata values
// Only bucket fields specified in request body will overwrite existing values.
func patchBucket(w http.ResponseWriter, r *http.Request) {

	pathParams := mux.Vars(r)
	sageBucketID := pathParams["bucket"]
	//objectKey := pathParams["key"]

	vars := mux.Vars(r)
	username := vars["username"]

	//rawQuery := r.URL.RawQuery

	// normal bucket metadata

	allowed, err := userHasBucketPermission(username, sageBucketID, "FULL_CONTROL")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !allowed {
		respondJSONError(w, http.StatusUnauthorized, "Write access to bucket metadata denied (%s, %s)", username, sageBucketID)
		return
	}

	var deltaBucket map[string]string

	err = json.NewDecoder(r.Body).Decode(&deltaBucket)
	if err != nil {
		err = fmt.Errorf("Could not parse json: %s", err.Error())
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("got: %v", deltaBucket)

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	newBucketname, ok := deltaBucket["name"]
	if ok {

		insertQueryStr := "UPDATE Buckets SET name=? WHERE  id=UUID_TO_BIN(?) ;"
		_, err = db.Exec(insertQueryStr, newBucketname, sageBucketID)
		if err != nil {
			err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
			return
		}

	}

	// return should return real bucket

	newBucket, err := GetSageBucket(sageBucketID)
	if err != nil {
		err = fmt.Errorf("GetSageBucket returned: %s", err.Error())
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, newBucket)
	//bucket fields:
	//metadata , name , type, (owner, change requires permission change)

	//permission

}

// putBucket only for permissions
func putBucket(w http.ResponseWriter, r *http.Request) {

	pathParams := mux.Vars(r)
	sageBucketID := pathParams["bucket"]
	//objectKey := pathParams["key"]

	vars := mux.Vars(r)
	username := vars["username"]

	rawQuery := r.URL.RawQuery

	if strings.Contains(rawQuery, "permission") {
		allowed, err := userHasBucketPermission(username, sageBucketID, "WRITE_ACP")
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !allowed {
			respondJSONError(w, http.StatusUnauthorized, "Write access to bucket permissions denied (%s, %s)", username, sageBucketID)
			return
		}

		bucketObject, err := GetSageBucket(sageBucketID)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		var newPerm SAGEBucketPermission

		err = json.NewDecoder(r.Body).Decode(&newPerm)
		if err != nil {
			err = fmt.Errorf("Could not parse json: %s", err.Error())
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		if bucketObject.Owner == newPerm.Grantee {
			respondJSONError(w, http.StatusBadRequest, "You cannot change your own permissons.")
			return
		}

		if newPerm.GranteeType == "" {
			respondJSONError(w, http.StatusBadRequest, "GranteeType missing")
			return
		}

		if newPerm.GranteeType == "GROUP" && newPerm.Grantee == "AllUsers" && newPerm.Permission != "READ" {
			respondJSONError(w, http.StatusBadRequest, "Buckets can be made public only for READ access.")
			return
		}

		//do something
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			err = fmt.Errorf("Unable to connect to database: %v", err)
			return
		}
		defer db.Close()

		insertQueryStr := "INSERT INTO BucketPermissions (id, granteeType, grantee, permission) VALUES ( UUID_TO_BIN(?), ? , ?, ?) ;"
		_, err = db.Exec(insertQueryStr, sageBucketID, newPerm.GranteeType, newPerm.Grantee, newPerm.Permission)
		if err != nil {
			me, ok := err.(*mysql.MySQLError)
			if ok {
				if me.Number == 1062 {
					// entry already exists, quietly respond OK
					respondJSON(w, http.StatusOK, newPerm)
					return
				}
			}
			err = fmt.Errorf("Adding bucket permissions failed: %s", err.Error())
			respondJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, newPerm)
		return
	}

	respondJSONError(w, http.StatusUnauthorized, "Only query ?permissions supported")
	return
	//respondJSON(w, http.StatusOK, newBucket)
	//bucket fields:
	//metadata , name , type, (owner, change requires permission change)

	//permission

}

// deleteBucket deletes bucket, files, and bucket permissions
func deleteBucket(w http.ResponseWriter, r *http.Request) {

	//pathParams := mux.Vars(r)
	//sageBucketID := pathParams["bucket"]
	//objectKey := pathParams["key"]

	vars := mux.Vars(r)
	username := vars["username"]

	sageBucketID := vars["bucket"]
	if len(sageBucketID) != 36 {
		respondJSONError(w, http.StatusInternalServerError, "bucket id (%s) invalid (%d)", sageBucketID, len(sageBucketID))
		return
	}

	_, sagePath, err := getSagePath(r.URL.Path)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return

	}

	rawQuery := r.URL.RawQuery

	if (sagePath == "") && strings.Contains(rawQuery, "permission") {
		allowed, err := userHasBucketPermission(username, sageBucketID, "WRITE_ACP")
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !allowed {
			respondJSONError(w, http.StatusUnauthorized, "Write access to bucket permissions denied (%s, %s)", username, sageBucketID)
			return
		}

		bucketObject, err := GetSageBucket(sageBucketID)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		values := r.URL.Query()
		grantees, ok := values["grantee"]
		if !ok {
			respondJSONError(w, http.StatusBadRequest, "query field \"grantee\" missing")
			return
		}

		if len(grantees) == 0 {
			respondJSONError(w, http.StatusBadRequest, "query field \"grantee\" missing")
			return
		}

		dr := DeleteRespsonse{}
		dr.Deleted = []string{}
		for _, grantee := range grantees {
			granteeType := "USER"
			granteeArray := strings.SplitN(grantee, ":", 3)
			deletePermission := ""
			if len(granteeArray) >= 2 {
				granteeType = granteeArray[0]
				grantee = granteeArray[1]
			}
			if len(granteeArray) == 3 {
				deletePermission = granteeArray[2]
			}

			if bucketObject.Owner == grantee {
				respondJSONError(w, http.StatusBadRequest, "You cannot change your own permissons.")
				return
			}

			//do something
			db, err := sql.Open("mysql", mysqlDSN)
			if err != nil {
				err = fmt.Errorf("Unable to connect to database: %v", err)
				return
			}
			defer db.Close()

			insertQueryStr := ""
			var result sql.Result
			if deletePermission == "" {
				insertQueryStr = "DELETE FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND granteeType=? AND grantee=? ;"
				result, err = db.Exec(insertQueryStr, sageBucketID, granteeType, grantee)
			} else {
				insertQueryStr = "DELETE FROM BucketPermissions WHERE id=UUID_TO_BIN(?) AND granteeType=? AND grantee=? AND permission=?;"
				result, err = db.Exec(insertQueryStr, sageBucketID, granteeType, grantee, deletePermission)
			}
			if err != nil {
				err = fmt.Errorf("Removing bucket permissions failed: %s", err.Error())
				respondJSONError(w, http.StatusUnauthorized, err.Error())
				return
			}

			deletedNumber, err := result.RowsAffected()
			if err != nil {
				err = fmt.Errorf("result.RowsAffected returned: %s", err.Error())
				respondJSONError(w, http.StatusUnauthorized, err.Error())
				return
			}

			if deletedNumber > 0 {
				if deletePermission == "" {
					dr.Deleted = append(dr.Deleted, granteeType+":"+grantee)
				} else {
					dr.Deleted = append(dr.Deleted, granteeType+":"+grantee+":"+deletePermission)
				}
			}
		}

		respondJSON(w, http.StatusOK, dr)
		return
	}

	// delete bucket or file

	allowed, err := userHasBucketPermission(username, sageBucketID, "WRITE")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !allowed {
		respondJSONError(w, http.StatusUnauthorized, "Write access to bucket denied (%s, %s)", username, sageBucketID)
		return
	}

	// delete bucket
	if sagePath == "" {
		// delete bucket and all its contents

		// 1) check if bucket exists
		sageBucket, err := GetSageBucket(sageBucketID)
		if err != nil {
			respondJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}
		_ = sageBucket

		// 2) delete files in bucket

		totalDeleted, err := deleteAllFiles(sageBucketID)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = totalDeleted

		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			err = fmt.Errorf("Unable to connect to database: %v", err)
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer db.Close()

		// 3) delete bucket
		insertQueryStr := "DELETE FROM Buckets  WHERE  id=UUID_TO_BIN(?) ;"
		_, err = db.Exec(insertQueryStr, sageBucketID)
		if err != nil {
			err = fmt.Errorf("Bucket creation in mysql failed: %s", err.Error())
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		data := DeleteRespsonse{}
		data.Deleted = []string{sageBucketID} // fmt.Sprintf("totalDeleted: %d", totalDeleted)

		respondJSON(w, http.StatusOK, data)
		return
	}

	// delete file

	deleted, err := deleteSAGEFiles(sageBucketID, []string{sagePath})
	if err != nil {
		err = fmt.Errorf("Deleting files failed: %s", err.Error())
		respondJSONError(w, http.StatusInternalServerError, err.Error())
	}

	data := DeleteRespsonse{}
	data.Deleted = deleted
	respondJSON(w, http.StatusOK, data)
	return
	//respondJSON(w, http.StatusOK, newBucket)
	//bucket fields:
	//metadata , name , type, (owner, change requires permission change)

	//permission

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

	s3BucketName := getS3BucketID(sageBucketID) //s3BucketPrefix + sageBucketID[0:2] // first two characters of uuid
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
				respondJSONError(w, http.StatusBadRequest, "part upload has no filename and no key was specified")
				return
			}
			s3Key = path.Join(preliminaryS3Key, filename)
			sageKey = path.Join(preliminarySageKey, filename)

		} else {
			s3Key = preliminaryS3Key
			sageKey = preliminarySageKey
		}
		sageKey = strings.TrimPrefix(sageKey, "/")
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
		fileUploadCounter.Inc()
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
	sageBucketID := getS3BucketID(uuidStr) //s3BucketPrefix + uuidStr[0:2]
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
	vars["username"] = ""

	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		next(w, r)
		//respondJSONError(w, http.StatusInternalServerError, "Authorization header is missing")
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

	if disableAuth {
		if strings.HasPrefix(tokenStr, "user:") {
			username := strings.TrimPrefix(tokenStr, "user:")
			vars["username"] = username
		} else {
			vars["username"] = "user-auth-disabled"
		}

		next(w, r)
		return
	}

	url := tokenInfoEndpoint

	log.Printf("url: %s", url)

	payload := strings.NewReader("token=" + tokenStr)
	client := &http.Client{
		Timeout: time.Second * 5,
	}
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		log.Print("NewRequest returned: " + err.Error())
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, err.Error())
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
		log.Print(err)
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.StatusCode != 200 {
		fmt.Printf("%s", body)
		//http.Error(w, fmt.Sprintf("token introspection failed (%d) (%s)", res.StatusCode, body), http.StatusInternalServerError)
		respondJSONError(w, http.StatusUnauthorized, fmt.Sprintf("token introspection failed (%d) (%s)", res.StatusCode, body))
		return
	}

	var dat map[string]interface{}
	if err := json.Unmarshal(body, &dat); err != nil {
		//fmt.Println(err)
		//http.Error(w, err.Error(), http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	val, ok := dat["error"]
	if ok && val != nil {
		fmt.Fprintf(w, val.(string)+"\n")

		//http.Error(w, val.(string), http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, val.(string))
		return

	}

	isActiveIf, ok := dat["active"]
	if !ok {
		//http.Error(w, "field active was misssing", http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, "field active missing")
		return
	}
	isActive, ok := isActiveIf.(bool)
	if !ok {
		//http.Error(w, "field active is noty a boolean", http.StatusInternalServerError)
		respondJSONError(w, http.StatusInternalServerError, "field active is not a boolean")
		return
	}

	if !isActive {
		//http.Error(w, "token not active", http.StatusInternalServerError)
		respondJSONError(w, http.StatusUnauthorized, "token not active")
		return
	}

	usernameIf, ok := dat["username"]
	if !ok {
		//respondJSONError(w, http.StatusInternalServerError, "username is missing")
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
