package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(
	w http.ResponseWriter,
	r *http.Request,
) {
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
		respondWithError(
			w,
			http.StatusUnauthorized,
			"Couldn't validate JWT",
			err,
		)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	const maxMemory = 10 << 30
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Unable to parse form file",
			err,
		)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(
			w,
			http.StatusBadRequest,
			"Unable to parse form file",
			err,
		)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(
			w,
			http.StatusBadRequest,
			"Missing Content-Type for video",
			nil,
		)
		return
	}

	mediaType, _, err = mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(
			w,
			http.StatusBadRequest,
			"Invalid Content-Type for video",
			err,
		)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(
			w,
			http.StatusUnsupportedMediaType,
			"Filetype must be mp4",
			nil,
		)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Unable to find video",
			err,
		)
		return
	} else if video.UserID != userID {
		respondWithError(
			w,
			http.StatusUnauthorized,
			"not authorized to modify video",
			nil,
		)
		return
	}

	extension := getExtensionFromType(mediaType)

	rando := make([]byte, 32)
	_, err = rand.Read(rando)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Unable to generate filename",
			err,
		)
		return
	}
	randoString := base64.RawURLEncoding.EncodeToString(rando)

	fileName := fmt.Sprintf("%s.%s", randoString, extension)

	tempFile, err := os.CreateTemp("", "tubely-temp."+extension)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to save file",
			err,
		)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to save file",
			err,
		)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to process file",
			err,
		)
		return
	}

	ratio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(
			w,
			http.StatusUnsupportedMediaType,
			"unable to get aspect ratio",
			err,
		)
	}

	if ratio == "16:9" {
		fileName = "landscape/" + fileName
	} else if ratio == "9:16" {
		fileName = "portrait/" + fileName
	} else {
		fileName = "other/" + fileName
	}

	optimizedFileName, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to optimize file",
			err,
		)
		return
	}
	defer os.Remove(optimizedFileName)

	optimizedFile, err := os.Open(optimizedFileName)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to load optimized file",
			err,
		)
		return
	}
	defer optimizedFile.Close()

	poi := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		Body:        optimizedFile,
		ContentType: &mediaType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &poi)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to upload file to s3",
			err,
		)
		return
	}

	url := fmt.Sprintf(
		"%s,%s",
		cfg.s3Bucket,
		fileName,
	)
	video.VideoURL = &url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to update database",
			err,
		)
		return
	}

	video, err = cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to generate link to video",
			err,
		)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
