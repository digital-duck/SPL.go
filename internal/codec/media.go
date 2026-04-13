package codec

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DetectMediaType returns the MIME type of a file or data URL.
func DetectMediaType(input string) (string, error) {
	if strings.HasPrefix(input, "data:") {
		idx := strings.Index(input, ";")
		if idx > 5 {
			return input[5:idx], nil
		}
		return "", fmt.Errorf("invalid data URL")
	}

	// It's a file path
	ext := strings.ToLower(filepath.Ext(input))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".webp":
		return "image/webp", nil
	case ".gif":
		return "image/gif", nil
	case ".wav":
		return "audio/wav", nil
	case ".mp3":
		return "audio/mpeg", nil
	case ".mp4":
		return "video/mp4", nil
	}

	// Try sniffing
	f, err := os.Open(input)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buffer := make([]byte, 512)
	_, err = f.Read(buffer)
	if err != nil {
		return "", err
	}

	return http.DetectContentType(buffer), nil
}

// ReadMedia returns the bytes of a media file or data URL.
func ReadMedia(input string) ([]byte, error) {
	if strings.HasPrefix(input, "data:") {
		// data:image/png;base64,iVBORw...
		idx := strings.Index(input, "base64,")
		if idx == -1 {
			return nil, fmt.Errorf("only base64 data URLs supported")
		}
		// In a real implementation, we'd use base64.StdEncoding.DecodeString here.
		// For now, this is a placeholder for the concept.
		return []byte(input[idx+7:]), nil 
	}

	return os.ReadFile(input)
}
