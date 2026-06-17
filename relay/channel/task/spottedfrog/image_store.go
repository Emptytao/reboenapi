package spottedfrog

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

const imageServePrefix = "/img"

func ensureImageDir() (string, error) {
	dir := filepath.Clean("img")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func publicImageURL(filename string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(common.TaskImagePublicBaseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("TaskImagePublicBaseURL is required for Grok image uploads")
	}
	filename = strings.TrimLeft(strings.TrimSpace(filename), "/")
	if filename == "" {
		return "", fmt.Errorf("empty filename")
	}
	return baseURL + imageServePrefix + "/" + filename, nil
}

func storeImageBytes(data []byte, contentType string) (string, error) {
	dir, err := ensureImageDir()
	if err != nil {
		return "", err
	}
	contentType = normalizeContentType(contentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	ext := ".bin"
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	filename := common.GetUUID() + ext
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func storeImageFromString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty image reference")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, nil
	}
	if strings.HasPrefix(raw, "data:image/") || strings.Contains(raw, ";base64,") {
		contentType, base64Data, err := service.DecodeBase64FileData(raw)
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64Data)))
		if err != nil {
			return "", err
		}
		filename, err := storeImageBytes(data, normalizeContentType(contentType))
		if err != nil {
			return "", err
		}
		return publicImageURL(filename)
	}
	contentType, base64Data, err := service.DecodeBase64FileData(raw)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64Data)))
	if err != nil {
		return "", err
	}
	filename, err := storeImageBytes(data, normalizeContentType(contentType))
	if err != nil {
		return "", err
	}
	return publicImageURL(filename)
}

func storeImageFromReader(r io.Reader, contentType string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	filename, err := storeImageBytes(data, normalizeContentType(contentType))
	if err != nil {
		return "", err
	}
	return publicImageURL(filename)
}
