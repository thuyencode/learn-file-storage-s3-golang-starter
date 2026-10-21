package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"slices"
	"uuid"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/thuyencode/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/thuyencode/learn-file-storage-s3-golang-starter/internal/database"
	v "github.com/thuyencode/learn-file-storage-s3-golang-starter/internal/video"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	log.Println("uploading video", videoID, "by user", userID)

	// 1 GB
	const maxMemory = 1 << 30
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

	formFile, header, err := r.FormFile("video")
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

	alllowedMediaTypes := []string{"video/mp4"}
	if !slices.Contains(alllowedMediaTypes, mediaType) {
		respondWithError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("The video file type must be one of %v", alllowedMediaTypes),
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

	tempFile, err := os.CreateTemp("", fileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	defer func() {
		if err := tempFile.Close(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	if _, err = io.Copy(tempFile, formFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	if _, err = tempFile.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	prefix, err := v.GetVideoAspectRatio(tempFile.Name())
	if err != nil {
		if errors.Is(err, v.ErrInvalidData) {
			respondWithError(w, http.StatusBadRequest, "Invalid file. Must be a video file.", err)
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	encodedFilePath, err := v.ProcessVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	encodedFile, err := os.Open(encodedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	defer func() {
		if err := os.Remove(encodedFilePath); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		}
	}()

	defer func() {
		if err := encodedFile.Close(); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
			return
		}
	}()

	s3ObjKey := path.Join(prefix, fileName)
	if _, err = cfg.s3Client.PutObject(
		context.Background(),
		&s3.PutObjectInput{
			Bucket:      &cfg.s3Bucket,
			Key:         &s3ObjKey,
			Body:        encodedFile,
			ContentType: &contentTypeHeader,
		},
	); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	video.VideoURL = new(fmt.Sprintf("%s,%s", cfg.s3Bucket, s3ObjKey))
	if err = cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	video, err = cfg.dbVideoToSignedVideo(r.Context(), video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
