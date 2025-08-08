package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const bucketFormatUrl = "https://%s.s3.%s.amazonaws.com/%s"
const videoSourceUrlLength = 32

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse video ID", nil)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse token", nil)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized access", err)
		return
	}

	fmt.Println("uploading content for video", videoID, "by user", userID)

	dbVideoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}

	if dbVideoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized access", nil)
		return
	}

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	vdFile, vdHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video file", err)
		return
	}
	defer vdFile.Close()

	fileName := vdHeader.Filename

	contentType := vdHeader.Header.Get("Content-Type")
	if contentType == "" {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video file type", nil)
		return
	}

	//mediaType, _, err := mime.ParseMediaType(contentType)
	//if err != nil {
	//	respondWithError(w, http.StatusBadRequest, "Couldn't parse MIME type", err)
	//	return
	//}
	//
	//if mediaType != "video/mp4" {
	//	respondWithError(w, http.StatusBadRequest, "File is not an MP4", nil)
	//}
	//
	//extensions, err := mime.ExtensionsByType(mediaType)
	//if err != nil {
	//	respondWithError(w, http.StatusBadRequest, "Couldn't get extensions", err)
	//	return
	//}
	//
	//vdExtension := ""
	//
	//for _, ex := range extensions {
	//	if ex == ".mp4" {
	//		vdExtension = ex
	//		break
	//	}
	//}
	//
	//if vdExtension == "" {
	//	vdExtension = extensions[0]
	//}

	tempVideoFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}

	tempVideoFilepath := tempVideoFile.Name()
	defer os.Remove(tempVideoFilepath)
	defer tempVideoFile.Close()

	_, err = io.Copy(tempVideoFile, vdFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save video", err)
		return
	}

	tempVideoFile.Seek(0, io.SeekStart)
	aspectRatioDescription, err := getVideoAspectRatio(tempVideoFilepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return
	}

	preProcessedVideoFilePath, err := processVideoForFastStart(tempVideoFilepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process video", err)
		return
	}

	preProcessedVideoFile, err := os.Open(preProcessedVideoFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open video", err)
		return
	}
	defer preProcessedVideoFile.Close()
	preProcessedVideoFile.Seek(0, io.SeekStart)

	//newPathBytes := make([]byte, videoSourceUrlLength)
	//_, err = rand.Read(newPathBytes)
	//if err != nil {
	//	respondWithError(w, http.StatusBadRequest, "Couldn't get random filename", err)
	//	return
	//}
	//
	//newVdFileTitle := base64.RawURLEncoding.EncodeToString(newPathBytes)
	//newVdFileName := fmt.Sprintf("%s%s", newVdFileTitle, vdExtension)

	newVdFileName := filepath.Join(aspectRatioDescription, fileName)
	newVdUrl := filepath.Join(cfg.s3CfDistribution, newVdFileName)

	updatedVideoMetadata := dbVideoMetadata
	updatedVideoMetadata.UpdatedAt = time.Now()
	updatedVideoMetadata.VideoURL = &newVdUrl

	err = cfg.db.UpdateVideo(updatedVideoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save video", err)
		return
	}

	putObjectInputParams := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &newVdFileName,
		Body:        preProcessedVideoFile,
		ContentType: &contentType,
	}

	_, err = cfg.s3Client.PutObject(context.Background(), &putObjectInputParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideoMetadata)
}
