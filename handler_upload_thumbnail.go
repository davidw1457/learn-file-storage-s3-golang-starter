package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	if mediaType == "" || !strings.HasPrefix(mediaType, "image/") {
		respondWithError(
			w,
			http.StatusBadRequest,
			"Missing Content-Type for thumbnail",
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

	extension := strings.TrimPrefix(mediaType, "image/")
	fileName := fmt.Sprintf("%s.%s", videoID, extension)
	path := filepath.Join(cfg.assetsRoot, fileName)

	thumbFile, err := os.Create(path)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to save file",
			err,
		)
	}

	_, err = io.Copy(thumbFile, file)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"unable to save file",
			err,
		)
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	video.ThumbnailURL = &url

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
