package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const maxMemory = 10 << 20
const assetsUrlFormat = "http://localhost:%s/assets/%s"
const thumbnailUrlByteLength = 32

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	tnFile, tnHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail file", err)
		return
	}
	defer tnFile.Close()

	contentType := tnHeader.Header.Get("Content-Type")
	if contentType == "" {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail file type", nil)
		return
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse MIME type", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Media is not a jpeg or png", nil)
		return
	}

	extensions, err := mime.ExtensionsByType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get extensions", err)
		return
	}

	tnExtension := extensions[0]
	newPathBytes := make([]byte, thumbnailUrlByteLength)
	_, err = rand.Read(newPathBytes)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get random filename", err)
		return
	}

	newTnFileTitle := base64.RawURLEncoding.EncodeToString(newPathBytes)

	newTnFileName := fmt.Sprintf("%s%s", newTnFileTitle, tnExtension)
	newTnFilePath := filepath.Join(cfg.assetsRoot, newTnFileName)
	newTnUrl := fmt.Sprintf(assetsUrlFormat, cfg.port, newTnFileName)

	imageFile, err := os.Create(newTnFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write image", err)
		return
	}

	_, err = io.Copy(imageFile, tnFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write image", err)
		return
	}

	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video metadata", err)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not authorized to upload this video", nil)
		return
	}

	updatedVideoMetadata := videoMetadata
	updatedVideoMetadata.UpdatedAt = time.Now()
	updatedVideoMetadata.ThumbnailURL = &newTnUrl

	err = cfg.db.UpdateVideo(updatedVideoMetadata)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, updatedVideoMetadata)
}
