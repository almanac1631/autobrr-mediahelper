package autobrrmediahelper

import (
	"errors"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
)

type MediaCheckRequest struct {
	Episode     string `json:"Episode"`
	FilterName  string `json:"FilterName"`
	Indexer     string `json:"Indexer"`
	Resolution  string `json:"Resolution"`
	Season      string `json:"Season"`
	Source      string `json:"Source"`
	Title       string `json:"Title" binding:"required"`
	TorrentName string `json:"TorrentName"`
	Type        string `json:"Type"`
	Year        int    `json:"Year" binding:"required"`
}

func mediaCheck(manager *MediaManager) func(c *gin.Context) {
	handler := func(c *gin.Context) {
		var req MediaCheckRequest
		if err := c.BindJSON(&req); err != nil {
			slog.Info("received invalid media check request", "error", err)
			return
		}
		slog.Info("received media check request. searching for imdb id...", "url", c.Request.URL, "request", req)
		mediaId, err := manager.GetMediaIdByNameAndYear(req.Title, req.Year)
		if errors.Is(err, MediaNotFoundErr) {
			c.String(http.StatusNotFound, "media not found")
			return
		} else if err != nil {
			slog.Error("failed to search for media", "error", err)
			c.String(http.StatusInternalServerError, "failed to search for media")
			return
		}
		slog.Info("found media", "id", mediaId)
		shouldBeDownloaded, err := manager.ShouldMediaBeDownloaded(mediaId)
		slog.Info("media download status retrieved", "id", mediaId, "shouldBeDownloaded", shouldBeDownloaded)
		if err != nil {
			slog.Error("failed to check if media should be downloaded", "error", err)
			c.String(http.StatusInternalServerError, "failed to check if media should be downloaded")
			return
		} else if !shouldBeDownloaded {
			c.String(http.StatusNotFound, "media should not be downloaded")
		} else {
			c.String(http.StatusOK, "media should be downloaded")
		}
	}
	return handler
}
