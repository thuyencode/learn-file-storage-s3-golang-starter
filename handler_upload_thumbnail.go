package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/uuid"
	"github.com/thuyencode/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/thuyencode/learn-file-storage-s3-golang-starter/internal/database"
)

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

	log.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse multipart form", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	if video == (database.Video{}) {
		respondWithError(w, http.StatusNotFound, "Video ID not found", err)
		return
	}

	if video.UserID != userID {
		respondWithJSON(w, http.StatusUnauthorized, "Not your video")
		return
	}

	formFile, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer func() {
		if err := formFile.Close(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	contentTypeHeader := header.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	alllowedMediaTypes := []string{"image/jpeg", "image/png"}
	if !slices.Contains(alllowedMediaTypes, mediaType) {
		respondWithError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("The thumbnail file type must be one of %v", alllowedMediaTypes),
			nil,
		)
		return
	}

	fileExtensions, _ := mime.ExtensionsByType(mediaType)
	fileName := fmt.Sprintf("%s%s", videoIDString, fileExtensions[0])
	filePath := filepath.Join(cfg.assetsRoot, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create a new file", err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	_, err = io.Copy(file, formFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to write a new file", err)
		return
	}

	video.ThumbnailURL = new(fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName))

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
