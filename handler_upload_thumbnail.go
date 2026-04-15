package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
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

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to parse form", err)
		return
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to get file", err)
		return
	}
	defer file.Close()

	mediaType := fileHeader.Header.Get("Content-Type")

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to upload thumbnail for this video", nil)
		return
	}

	var fileExtension string
	switch mediaType {
	case "image/jpeg":
		fileExtension = ".jpg"
	case "image/png":
		fileExtension = ".png"
	case "image/gif":
		fileExtension = ".gif"
	case "image/webp":
		fileExtension = ".webp"
	default:
		respondWithError(w, http.StatusBadRequest, "Unsupported image type", nil)
		return
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate random filename", err)
		return
	}
	randomFileName := base64.RawURLEncoding.EncodeToString(randomBytes)

	filePath := filepath.Join(cfg.assetsRoot, randomFileName+fileExtension)

	diskFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create file", err)
		return
	}
	defer diskFile.Close()

	_, err = io.Copy(diskFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save file", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s%s", cfg.port, randomFileName, fileExtension)
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}