package main

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "io"
    "mime"
    "net/http"
    "os"

    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
    "github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<30)

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

    video, err := cfg.db.GetVideo(videoID)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
        return
    }

    if video.UserID != userID {
        respondWithError(w, http.StatusUnauthorized, "Not authorized to upload video", nil)
        return
    }

    file, fileHeader, err := r.FormFile("video")
    if err != nil {
        respondWithError(w, http.StatusBadRequest, "Couldn't get video file", err)
        return
    }
    defer file.Close()

    mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
    if err != nil || mediaType != "video/mp4" {
        respondWithError(w, http.StatusBadRequest, "Invalid file type, only MP4 is allowed", err)
        return
    }

    tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
        return
    }
    defer os.Remove(tempFile.Name())
    defer tempFile.Close()

    _, err = io.Copy(tempFile, file)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't save temp file", err)
        return
    }

    _, err = tempFile.Seek(0, io.SeekStart)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't seek temp file", err)
        return
    }

    randomBytes := make([]byte, 32)
    _, err = rand.Read(randomBytes)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't generate filename", err)
        return
    }
    randomFileName := hex.EncodeToString(randomBytes)

    aspectRatio, err := getVideoAspectRatio(tempFile.Name())
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
        return
    }

    var prefix string
    switch aspectRatio {
    case "16:9":
        prefix = "landscape/"
    case "9:16":
        prefix = "portrait/"
    default:
        prefix = "other/"
    }

    fileKey := prefix + randomFileName + ".mp4"

    processedFilePath, err := processVideoForFastStart(tempFile.Name())
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't process video", err)
        return
    }
    defer os.Remove(processedFilePath)

    processedFile, err := os.Open(processedFilePath)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't open processed file", err)
        return
    }
    defer processedFile.Close()

    _, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
        Bucket:      &cfg.s3Bucket,
        Key:         &fileKey,
        Body:        processedFile,
        ContentType: &mediaType,
    })
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't upload to S3", err)
        return
    }

    cloudFrontURL := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, fileKey)
    video.VideoURL = &cloudFrontURL
    err = cfg.db.UpdateVideo(video)
    if err != nil {
        respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
        return
    }

    respondWithJSON(w, http.StatusOK, video)
}