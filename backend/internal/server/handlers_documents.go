package server

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"registre-admin/internal/repository"
	"registre-admin/internal/types"
)

// UploadDocumentHandler uploads a document file with metadata.
// The MIME type is detected from the uploaded file. Storage backend is determined by STORAGE_TYPE env var.
//
// @Summary     Upload a document
// @Description Uploads a document file with metadata (multipart form). Detects MIME type automatically.
// @Tags        documents
// @Accept      multipart/form-data
// @Produce     json
// @Param       name      formData string true  "Document display name"
// @Param       type_code formData string true  "Document type code"
// @Param       file      formData file   true  "File to upload"
// @Success     201 {object} types.Document
// @Failure     400 {object} types.ErrorResponse
// @Failure     413 {object} types.ErrorResponse
// @Failure     500 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /documents [post]
func UploadDocumentHandler(docRepo repository.DocumentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !parseUploadForm(c) {
			return
		}
		name := c.PostForm("name")
		typeCode := c.PostForm("type_code")

		if name == "" || typeCode == "" {
			handleBadRequest(c, "name and type_code are required")
			return
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			handleBadRequest(c, "file is required")
			return
		}

		const maxUploadSize = 10 << 20 // 10 MB
		if fileHeader.Size > maxUploadSize {
			handleBadRequest(c, "File size exceeds 10 MB limit")
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			handleInternalError(c, err, "Failed to read file")
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			handleInternalError(c, err, "Failed to read file")
			return
		}

		// Detect MIME type from upload header or file extension
		mimeType := fileHeader.Header.Get("Content-Type")
		if mimeType == "" || mimeType == "application/octet-stream" {
			mimeType = mime.TypeByExtension(filepath.Ext(fileHeader.Filename))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
		}

		// Sanitize filename: strip directory components and quotes
		fileName := filepath.Base(fileHeader.Filename)
		fileName = strings.ReplaceAll(fileName, "\"", "")
		if name == "" {
			name = fileName
		}

		doc := types.Document{
			Name:     name,
			FileName: fileName,
			TypeCode: typeCode,
			MimeType: mimeType,
		}

		if err := docRepo.Create(&doc, fileBytes); err != nil {
			handleInternalError(c, err, "Failed to create document")
			return
		}

		c.JSON(http.StatusCreated, doc)
	}
}

// DownloadDocumentHandler serves a document file by ID.
// For database storage, serves the file directly. For S3, redirects to the storage URL.
//
// @Summary     Download a document file
// @Description Serves the document file as an attachment. Redirects to S3 URL if stored externally.
// @Tags        documents
// @Produce     octet-stream
// @Param       id path int true "Document ID"
// @Success     200 {file}   binary
// @Success     307 {string} string "Redirect to S3 URL"
// @Failure     400 {object} types.ErrorResponse
// @Failure     404 {object} types.ErrorResponse
// @Security BearerAuth
// @Router      /documents/{id}/file [get]
func DownloadDocumentHandler(docRepo repository.DocumentDAO) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parsePathID(c, "id", "document")
		if !ok {
			return
		}

		fileBytes, doc, err := docRepo.GetFile(id)
		if err != nil {
			handleNotFound(c, "Document")
			return
		}

		// S3 storage: redirect to the object store URL
		if doc.StorageType == "s3" && doc.StorageURL != nil {
			c.Redirect(http.StatusTemporaryRedirect, *doc.StorageURL)
			return
		}

		if len(fileBytes) == 0 {
			handleNotFound(c, "Document file")
			return
		}

		safeName := filepath.Base(doc.FileName)
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": safeName}))
		c.Data(http.StatusOK, doc.MimeType, fileBytes)
	}
}
