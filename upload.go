package main

import (
	"crypto/rand"
	"encoding/base64"
	"mime/multipart"
	"path/filepath"
)

func GenerateFileName() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func GenerateFileNameFromHeader(h *multipart.FileHeader) string {
	fileName := GenerateFileName()
	return fileName + filepath.Ext(h.Filename)
}
