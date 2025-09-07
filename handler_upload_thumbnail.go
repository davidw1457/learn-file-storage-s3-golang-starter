package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
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

	file, header, err := r.FormFile("thumbnail")
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
			"Missing Content-Type for thumbnail",
			nil,
		)
		return
	}

	fileContents, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read file", err)
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

	thumbnailURL := fmt.Sprintf(
		"data:%s;base64,%s",
		mediaType,
		base64.StdEncoding.EncodeToString(fileContents),
	)
	video.ThumbnailURL = &thumbnailURL

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

	respondWithJSON(w, http.StatusOK, video)
}
