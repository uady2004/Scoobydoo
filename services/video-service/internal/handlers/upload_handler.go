package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/tiktok-clone/video-service/internal/models"
	"github.com/tiktok-clone/video-service/internal/services"
)

// UploadHandler handles HTTP requests for the chunked upload API.
type UploadHandler struct {
	uploadSvc *services.UploadService
	logger    *zap.Logger
}

// NewUploadHandler constructs an UploadHandler.
func NewUploadHandler(uploadSvc *services.UploadService, logger *zap.Logger) *UploadHandler {
	return &UploadHandler{uploadSvc: uploadSvc, logger: logger}
}

// RegisterRoutes attaches upload routes to the given router group.
//
//	POST   /uploads/initiate          — start a new chunked upload
//	POST   /uploads/:uploadID/chunks  — upload one chunk
//	POST   /uploads/:uploadID/complete — assemble and finalise upload
//	GET    /uploads/:uploadID/progress — return current progress
//	GET    /uploads/:uploadID/resume   — return missing chunk list
//
// Flat aliases (uploadID in body/query, used by Flutter client via gateway):
//
//	POST   /uploads/chunk    — upload_id in form field
//	POST   /uploads/complete — upload_id in JSON body field
//	GET    /uploads/progress — upload_id in query param
//	GET    /uploads/resume   — upload_id in query param
func (h *UploadHandler) RegisterRoutes(rg *gin.RouterGroup) {
	uploads := rg.Group("/uploads")
	{
		uploads.POST("/initiate", h.InitiateUpload)
		uploads.POST("/:uploadID/chunks", h.UploadChunk)
		uploads.POST("/:uploadID/complete", h.CompleteUpload)
		uploads.GET("/:uploadID/progress", h.GetProgress)
		uploads.GET("/:uploadID/resume", h.ResumeUpload)

		// Flat aliases: uploadID comes from body/query instead of path segment
		uploads.POST("/chunk", h.UploadChunkFlat)
		uploads.POST("/complete", h.CompleteUploadFlat)
		uploads.GET("/progress", h.GetProgressFlat)
		uploads.GET("/resume", h.ResumeUploadFlat)
	}
}

// InitiateUpload godoc
// @Summary  Start a new chunked upload session
// @Tags     uploads
// @Accept   json
// @Produce  json
// @Param    body  body  models.InitiateUploadRequest  true  "Upload parameters"
// @Success  201   {object}  models.InitiateUploadResponse
// @Failure  400   {object}  ErrorResponse
// @Router   /uploads/initiate [post]
func (h *UploadHandler) InitiateUpload(c *gin.Context) {
	userID := extractUserID(c)
	if userID == "" {
		respondError(c, http.StatusUnauthorized, "missing user identity")
		return
	}

	var req models.InitiateUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	resp, err := h.uploadSvc.InitiateUpload(c.Request.Context(), &req, userID)
	if err != nil {
		h.logger.Error("InitiateUpload failed", zap.String("user_id", userID), zap.Error(err))
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// UploadChunk godoc
// @Summary  Upload a single chunk of a video
// @Tags     uploads
// @Accept   multipart/form-data
// @Produce  json
// @Param    uploadID    path      string  true   "Upload session ID"
// @Param    chunk_index formData  int     true   "Zero-based chunk index"
// @Param    chunk       formData  file    true   "Chunk binary data"
// @Success  200  {object}  map[string]interface{}
// @Failure  400  {object}  ErrorResponse
// @Router   /uploads/{uploadID}/chunks [post]
func (h *UploadHandler) UploadChunk(c *gin.Context) {
	uploadID := c.Param("uploadID")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "uploadID is required")
		return
	}

	// Determine the chunk index. The client may send it as a form field or
	// as the X-Chunk-Index header.
	chunkIndex, err := resolveChunkIndex(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Accept either multipart/form-data or raw body.
	var (
		data io.Reader
		size int64
	)

	contentType := c.GetHeader("Content-Type")
	switch {
	case isMultipart(contentType):
		file, header, formErr := c.Request.FormFile("chunk")
		if formErr != nil {
			respondError(c, http.StatusBadRequest, fmt.Sprintf("form file error: %v", formErr))
			return
		}
		defer file.Close()
		data = file
		size = header.Size

	default:
		// Raw binary body.
		data = c.Request.Body
		size = c.Request.ContentLength
		if size <= 0 {
			respondError(c, http.StatusBadRequest, "Content-Length must be set for raw chunk upload")
			return
		}
	}

	if err := h.uploadSvc.UploadChunk(c.Request.Context(), uploadID, chunkIndex, data, size); err != nil {
		h.logger.Error("UploadChunk failed",
			zap.String("upload_id", uploadID),
			zap.Int("chunk_index", chunkIndex),
			zap.Error(err),
		)
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id":   uploadID,
		"chunk_index": chunkIndex,
		"status":      "received",
	})
}

// CompleteUpload godoc
// @Summary  Finalise a chunked upload by assembling all chunks
// @Tags     uploads
// @Produce  json
// @Param    uploadID  path  string  true  "Upload session ID"
// @Success  200  {object}  models.Video
// @Failure  400  {object}  ErrorResponse
// @Router   /uploads/{uploadID}/complete [post]
func (h *UploadHandler) CompleteUpload(c *gin.Context) {
	uploadID := c.Param("uploadID")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "uploadID is required")
		return
	}

	video, err := h.uploadSvc.CompleteUpload(c.Request.Context(), uploadID)
	if err != nil {
		h.logger.Error("CompleteUpload failed",
			zap.String("upload_id", uploadID),
			zap.Error(err),
		)
		if isClientError(err) {
			respondError(c, http.StatusBadRequest, err.Error())
		} else {
			respondError(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"video":   video,
		"message": "upload complete; transcoding in progress",
	})
}

// GetProgress godoc
// @Summary  Get the current upload progress
// @Tags     uploads
// @Produce  json
// @Param    uploadID  path  string  true  "Upload session ID"
// @Success  200  {object}  models.UploadProgress
// @Failure  404  {object}  ErrorResponse
// @Router   /uploads/{uploadID}/progress [get]
func (h *UploadHandler) GetProgress(c *gin.Context) {
	uploadID := c.Param("uploadID")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "uploadID is required")
		return
	}

	progress, err := h.uploadSvc.GetUploadProgress(c.Request.Context(), uploadID)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, progress)
}

// ResumeUpload godoc
// @Summary  Return the list of missing chunks so a client can resume
// @Tags     uploads
// @Produce  json
// @Param    uploadID  path  string  true  "Upload session ID"
// @Success  200  {object}  models.UploadProgress
// @Failure  404  {object}  ErrorResponse
// @Router   /uploads/{uploadID}/resume [get]
func (h *UploadHandler) ResumeUpload(c *gin.Context) {
	uploadID := c.Param("uploadID")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "uploadID is required")
		return
	}

	progress, err := h.uploadSvc.ResumeUpload(c.Request.Context(), uploadID)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, progress)
}

// ---- Flat aliases (uploadID from body/query, not URL path) ------------------

// UploadChunkFlat handles POST /uploads/chunk — upload_id comes from the
// multipart form field instead of the URL path segment.
func (h *UploadHandler) UploadChunkFlat(c *gin.Context) {
	uploadID := c.PostForm("upload_id")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "upload_id form field is required")
		return
	}

	chunkIndex, err := resolveChunkIndex(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var (
		data io.Reader
		size int64
	)
	contentType := c.GetHeader("Content-Type")
	switch {
	case isMultipart(contentType):
		file, header, formErr := c.Request.FormFile("chunk")
		if formErr != nil {
			respondError(c, http.StatusBadRequest, fmt.Sprintf("form file error: %v", formErr))
			return
		}
		defer file.Close()
		data = file
		size = header.Size
	default:
		data = c.Request.Body
		size = c.Request.ContentLength
		if size <= 0 {
			respondError(c, http.StatusBadRequest, "Content-Length must be set for raw chunk upload")
			return
		}
	}

	if err := h.uploadSvc.UploadChunk(c.Request.Context(), uploadID, chunkIndex, data, size); err != nil {
		h.logger.Error("UploadChunkFlat failed",
			zap.String("upload_id", uploadID),
			zap.Int("chunk_index", chunkIndex),
			zap.Error(err),
		)
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Return progress so the Flutter client can show a progress bar.
	progress, _ := h.uploadSvc.GetUploadProgress(c.Request.Context(), uploadID)
	var pct float64
	if progress != nil {
		pct = progress.PercentDone
	}
	c.JSON(http.StatusOK, gin.H{
		"upload_id":   uploadID,
		"chunk_index": chunkIndex,
		"progress":    pct,
		"status":      "received",
	})
}

// CompleteUploadFlat handles POST /uploads/complete — upload_id in JSON body.
func (h *UploadHandler) CompleteUploadFlat(c *gin.Context) {
	var body struct {
		UploadID    string `json:"upload_id"`
		TotalChunks int    `json:"total_chunks"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UploadID == "" {
		respondError(c, http.StatusBadRequest, "upload_id is required in request body")
		return
	}

	video, err := h.uploadSvc.CompleteUpload(c.Request.Context(), body.UploadID)
	if err != nil {
		h.logger.Error("CompleteUploadFlat failed",
			zap.String("upload_id", body.UploadID),
			zap.Error(err),
		)
		if isClientError(err) {
			respondError(c, http.StatusBadRequest, err.Error())
		} else {
			respondError(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"video":   video,
		"message": "upload complete; transcoding in progress",
	})
}

// GetProgressFlat handles GET /uploads/progress?upload_id=<id>.
func (h *UploadHandler) GetProgressFlat(c *gin.Context) {
	uploadID := c.Query("upload_id")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "upload_id query parameter is required")
		return
	}

	progress, err := h.uploadSvc.GetUploadProgress(c.Request.Context(), uploadID)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	// Map percent_done → progress for Flutter client compatibility.
	c.JSON(http.StatusOK, gin.H{
		"progress":  progress.PercentDone,
		"status":    progress.Status,
		"upload_id": progress.UploadID,
	})
}

// ResumeUploadFlat handles GET /uploads/resume?upload_id=<id>.
func (h *UploadHandler) ResumeUploadFlat(c *gin.Context) {
	uploadID := c.Query("upload_id")
	if uploadID == "" {
		respondError(c, http.StatusBadRequest, "upload_id query parameter is required")
		return
	}

	progress, err := h.uploadSvc.ResumeUpload(c.Request.Context(), uploadID)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"missing_chunks": progress.MissingChunks,
		"upload_id":      progress.UploadID,
		"status":         progress.Status,
	})
}

// ---- shared helpers ---------------------------------------------------------

// resolveChunkIndex reads the chunk index from the X-Chunk-Index header or the
// chunk_index form field.
func resolveChunkIndex(c *gin.Context) (int, error) {
	// Header takes precedence.
	if hdr := c.GetHeader("X-Chunk-Index"); hdr != "" {
		idx, err := strconv.Atoi(hdr)
		if err != nil {
			return 0, fmt.Errorf("invalid X-Chunk-Index header: %v", err)
		}
		return idx, nil
	}
	// Fall back to form field.
	val := c.PostForm("chunk_index")
	if val == "" {
		return 0, errors.New("chunk_index is required (form field or X-Chunk-Index header)")
	}
	idx, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid chunk_index: %v", err)
	}
	return idx, nil
}

func isMultipart(ct string) bool {
	for _, part := range []string{"multipart/form-data"} {
		if len(ct) >= len(part) && ct[:len(part)] == part {
			return true
		}
	}
	return false
}

// isClientError returns true for errors that indicate a bad client request
// (missing chunks, bad parameters, etc.) vs. an internal server error.
func isClientError(err error) bool {
	msg := err.Error()
	for _, token := range []string{"missing", "not found", "out of range", "already"} {
		if containsCI(msg, token) {
			return true
		}
	}
	return false
}

func containsCI(s, substr string) bool {
	ls, lsub := len(s), len(substr)
	if lsub > ls {
		return false
	}
	for i := 0; i <= ls-lsub; i++ {
		match := true
		for j := 0; j < lsub; j++ {
			a, b := s[i+j], substr[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
