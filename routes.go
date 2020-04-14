package main

import (
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

// Data simple response object
type Data struct {
	ID       string             `json:"bucket-id,omitempty"`
	Key      string             `json:"key,omitempty"`
	Metadata map[string]*string `json:"metadata,omitempty"`
	Error    string             `json:"error,omitempty"`
}

func uploadObject(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	username := vars["username"]

	log.Printf("username: %s", username)

	data := Data{}

	query := r.URL.Query()
	dataTypeArray, ok := query["type"]

	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "Please specify data type via query field \"type\"")
		return
	}

	if len(dataTypeArray) == 0 {
		respondJSONError(w, http.StatusInternalServerError, "Please specify data type via query field \"type\"")
		return
	}

	dataType := dataTypeArray[0]

	if dataType == "" {
		respondJSONError(w, http.StatusInternalServerError, "Please specify data type via query field \"type\"")
		return
	}
	dataType = strings.ToLower(dataType)
	_, ok = validDataTypes[dataType]
	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "Data type %s not supported", dataType)

		return

	}

	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "ParseMultipartForm returned: %s", err.Error())
		return
	}

	newUUID, err := uuid.NewRandom()
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "error generateing uuid %s", err.Error())
		return

	}

	newUUIDStr := newUUID.String()

	data.ID = newUUIDStr
	objectName := newUUIDStr

	bucketName := "sagedata-" + newUUIDStr[0:2] // first two characters of uuid
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
				respondJSONError(w, http.StatusInternalServerError, "Unable to create bucket %q, %v", bucketName, err)
				return
			}

			log.Printf("bucket %s created", bucketName)
		}

	}

	// Wait until bucket is created before finishing

	//bucketName := dataType
	file, header, err := r.FormFile("file")
	//objectName := header.Filename
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	var objectMetadata map[string]*string
	objectMetadata = make(map[string]*string)

	data.Metadata = objectMetadata
	filename := header.Filename

	log.Printf("filename: %s", filename)
	if filename == "" {
		respondJSONError(w, http.StatusInternalServerError, "Filename missing")
		return
	}
	key := filename // use filename unless key has been specified

	objectMetadata["owner"] = &username
	objectMetadata["type"] = &dataType

	data.Key = key

	upParams := &s3manager.UploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(objectName + "/" + filename),
		Body:     file,
		Metadata: objectMetadata,
	}
	uploader := s3manager.NewUploader(newSession)
	_, err = uploader.Upload(upParams)
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", file, bucketName, err)
		return
	}

	//log.Printf("Upload - Bucket: %v and Object: %v\n", bucketName, objectName)

	respondJSON(w, http.StatusOK, data)
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

	w.WriteHeader(http.StatusOK)
}

func respondJSON(w http.ResponseWriter, statusCode int, data Data) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondJSONError(w http.ResponseWriter, statusCode int, msg string, args ...interface{}) {
	errorStr := fmt.Sprintf(msg, args...)
	respondJSON(w, statusCode, Data{Error: errorStr})
}

func authMW(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

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

	vars := mux.Vars(r)

	vars["username"] = username

	next(w, r)

}
