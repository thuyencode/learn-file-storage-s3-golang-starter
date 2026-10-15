package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"

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

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	mediaType := header.Header.Get("Content-Type")
	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read file", err)
		return
	}

	base64Thumbnail := base64.StdEncoding.EncodeToString(data)
	video.ThumbnailURL = new(fmt.Sprintf("data:%s;base64,%s", mediaType, base64Thumbnail))

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
