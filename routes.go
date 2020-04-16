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
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gorilla/mux"

	"github.com/google/uuid"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

func listByBucket(w http.ResponseWriter, r *http.Request) {
	result, err := svc.ListBuckets(nil)
	if err != nil {
		exitErrorf("Unable to list buckets, %v", err)
	}

	fmt.Fprintf(w, "Buckets:\n")

	for _, b := range result.Buckets {
		fmt.Fprintf(w, "* %s created on %s\n",
			aws.StringValue(b.Name), aws.TimeValue(b.CreationDate))
	}
	w.WriteHeader(http.StatusOK)
}

func getObjectFromBucket(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	bucketName := pathParams["bucket"]
	objectName := pathParams["object"]

	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &objectName,
	})
	defer out.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	Openfile, err := os.Create(objectName)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(Openfile, out.Body)
	if err != nil {
		log.Fatal(err)
	}
	FileStat, _ := Openfile.Stat()
	FileSize := strconv.FormatInt(FileStat.Size(), 10)
	w.Header().Set("Content-Disposition", "attachment; filename="+objectName)
	w.Header().Set("Content-Length", FileSize)
	http.ServeContent(w, r, Openfile.Name(), *out.LastModified, Openfile)
	fmt.Fprintf(w, "Download - Bucket: %v, Object: %v\n", bucketName, objectName)
	w.WriteHeader(http.StatusOK)
}

func putObjectInBucket(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "body not parsed"}`))
		return
	}
	bucketName := r.FormValue("bucket")
	file, header, err := r.FormFile("file")
	objectName := header.Filename
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	upParams := &s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	}
	uploader := s3manager.NewUploader(newSession)
	_, err = uploader.Upload(upParams)
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", file, bucketName, err)
	}

	fmt.Fprintf(w, "Upload - Bucket: %v and Object: %v\n", bucketName, objectName)
	w.WriteHeader(http.StatusOK)

}

// SAGEBucket _
type SAGEBucket struct {
	ID       string            `json:"id,omitempty"`
	Name     string            `json:"name,omitempty"` // optional name (might be indentical to only file in bucket) username/bucketname
	Metadata map[string]string `json:"metadata,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// SageFile simple response object
type SageFile struct {
	Bucket string `json:"bucket-id,omitempty"`
	Key    string `json:"key,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ErrorStruct _
type ErrorStruct struct {
	Error string `json:"error"`
}

func createBucketForUser(username string, dataType string) (bucketID string, err error) {

	newUUID, err := uuid.NewRandom()
	if err != nil {
		err = fmt.Errorf("error generateing uuid %s", err.Error())
		return
	}

	bucketID = newUUID.String()

	bucketName := "sagedata-" + bucketID[0:2] // first two characters of uuid
	log.Printf("bucketName: %s", bucketName)

	_, err = svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Printf("bucket creation error: %s ", err.Error())
		// skip creation if it already exists
		if !strings.HasPrefix(err.Error(), s3.ErrCodeBucketAlreadyOwnedByYou) {

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

	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		err = fmt.Errorf("Unable to connect to database: %v", err)
		return
	}
	defer db.Close()

	resultArray := make([]string, 1)
	queryStr := "SELECT COUNT(*) FROM Buckets WHERE id=UUID_TO_BIN(?);"
	row := db.QueryRow(queryStr, bucketID)

	err = row.Scan(resultArray)
	if err != nil {
		err = fmt.Errorf("Unable to query db: %v", err)
		return
	}

	log.Printf("buckets: %s", resultArray[0])

	return
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

	data := SAGEBucket{}

	vars := mux.Vars(r)
	username := vars["username"]
	log.Printf("username: %s", username)

	dataType, err := getQueryField(r, "type")
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, err.Error(), dataType)
		return
	}

	bucketName, err := getQueryField(r, "name")
	if err == nil {
		data.Name = bucketName
	} else {
		err = nil
	}

	bucketID, err := createBucketForUser(username, dataType)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "bucket creation failed: %s", err.Error())
		return
	}
	// TODO store owner info in mysql
	data.ID = bucketID

	respondJSON(w, http.StatusOK, data)

}

// PUT /objects/{id}/key...     bucket already exists
func uploadObjectDEPRECATED(w http.ResponseWriter, r *http.Request) { // DEPRECATED DEPRECATED DEPRECATED DEPRECATED

	pathParams := mux.Vars(r)
	sageBucketName := pathParams["bucket"]
	//objectKey := pathParams["key"]

	vars := mux.Vars(r)
	username := vars["username"]

	log.Printf("r.URL.Path: %s", r.URL.Path)
	// example: /api/v1/objects/cbc2c709-2ef7-4852-8f5e-038fdc7f2304/test3/test3.jpg

	pathParsed := strings.SplitN(r.URL.Path, "/", 6)

	if len(pathParsed) != 6 {
		respondJSONError(w, http.StatusBadRequest, "error parsing URL (%d)", len(pathParsed))
		return
	}

	if pathParsed[3] != "objects" {
		respondJSONError(w, http.StatusBadRequest, "error parsing URL (%s)", pathParsed[3])
		return
	}

	// bucket
	if pathParsed[4] != sageBucketName {
		respondJSONError(w, http.StatusBadRequest, "error parsing URL (%s , expected %s)", pathParsed[4], sageBucketName)
		return
	}

	sageKey := pathParsed[5]

	log.Printf("sageKey: %s", sageKey)

	s3Key := path.Join(sageBucketName, sageKey)
	log.Printf("s3Key: %s", s3Key)

	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "ParseMultipartForm returned: %s", err.Error())
		return
	}

	s3BucketName := "sagedata-" + sageBucketName[0:2] // first two characters of uuid
	log.Printf("s3BucketName: %s", s3BucketName)

	//bucketName := dataType
	file, header, err := r.FormFile("file")
	//objectName := header.Filename
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "FormFile error %s", err.Error())
		return
	}
	defer file.Close()

	var objectMetadata map[string]*string
	objectMetadata = make(map[string]*string)

	data := SageFile{}
	//data.Metadata = objectMetadata

	if false {
		filename := header.Filename

		log.Printf("filename: %s", filename)
		if filename == "" {
			respondJSONError(w, http.StatusInternalServerError, "Filename missing")
			return
		}
	}

	//key := filename // use filename unless key has been specified

	objectMetadata["owner"] = &username
	//objectMetadata["type"] = &dataType

	data.Key = sageKey

	upParams := &s3manager.UploadInput{
		Bucket:   aws.String(s3BucketName),
		Key:      aws.String(s3Key),
		Body:     file,
		Metadata: objectMetadata,
	}
	uploader := s3manager.NewUploader(newSession)
	_, err = uploader.Upload(upParams)
	if err != nil {
		// Print the error and exit.
		respondJSONError(w, http.StatusInternalServerError, "Upload to S3 backend failed: %s", err.Error())
		return
	}

	data.Bucket = sageBucketName
	//log.Printf("Upload - Bucket: %v and Object: %v\n", bucketName, objectName)
	log.Printf("user upload successful")

	respondJSON(w, http.StatusOK, data)
}

// PUT /objects/{bucket-id}/key... (woth or without filename in key)
func uploadObject(w http.ResponseWriter, r *http.Request) {

	pathParams := mux.Vars(r)
	sageBucketName := pathParams["bucket"]
	//objectKey := pathParams["key"]

	vars := mux.Vars(r)
	username := vars["username"]

	log.Printf("r.URL.Path: %s", r.URL.Path)
	// example: /api/v1/objects/cbc2c709-2ef7-4852-8f5e-038fdc7f2304/test3/test3.jpg

	pathParsed := strings.SplitN(r.URL.Path, "/", 6)

	if len(pathParsed) != 6 {
		respondJSONError(w, http.StatusBadRequest, "error parsing URL (%d)", len(pathParsed))
		return
	}

	if pathParsed[3] != "objects" {
		respondJSONError(w, http.StatusBadRequest, "error parsing URL (%s)", pathParsed[3])
		return
	}

	// bucket
	if pathParsed[4] != sageBucketName {
		respondJSONError(w, http.StatusBadRequest, "error parsing URL (%s , expected %s)", pathParsed[4], sageBucketName)
		return
	}

	preliminarySageKey := pathParsed[5]

	isDirectory := false
	if strings.HasSuffix(preliminarySageKey, "/") {
		isDirectory = true
	}

	log.Printf("preliminarySageKey: %s", preliminarySageKey)

	preliminaryS3Key := path.Join(sageBucketName, preliminarySageKey)
	log.Printf("preliminaryS3Key: %s", preliminaryS3Key)

	s3BucketName := "sagedata-" + sageBucketName[0:2] // first two characters of uuid
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

		data.Bucket = sageBucketName
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
	bucketName := "sagedata-" + uuidStr[0:2]
	key := path.Join(uuidStr, sageKey)

	log.Printf("bucketName: %s key: %s", bucketName, key)

	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &key,
	})
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Error getting data, svc.GetObject returned: %s", err.Error())
		return
	}
	defer out.Body.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+sageKey)
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
		w.Write(buffer)
		//fmt.Println(string(p[:n]))
	}

}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
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
