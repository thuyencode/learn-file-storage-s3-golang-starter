package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"uuid"

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
	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}
	fileName := fmt.Sprintf(
		"%s%s",
		base64.RawURLEncoding.EncodeToString(randBytes),
		fileExtensions[0],
	)
	filePath := filepath.Join(cfg.assetsRoot, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	if _, err = io.Copy(file, formFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	video.ThumbnailURL = new(fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName))

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
