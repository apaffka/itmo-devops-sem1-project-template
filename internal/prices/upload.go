package prices

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func ExtractUploadToTempFile(r *http.Request, maxUploadMB int64) (string, func(), error) {
	maxBytes := maxUploadMB * 1024 * 1024

	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)

	ct := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(ct)

	var reader io.Reader = r.Body

	if strings.HasPrefix(mediaType, "multipart/") {
		if err := r.ParseMultipartForm(maxBytes); err != nil {
			return "", nil, fmt.Errorf("multipart parse: %w", err)
		}
		if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
			return "", nil, errors.New("multipart: no file parts found")
		}

		var fh *multipart.FileHeader
		if fhs := r.MultipartForm.File["file"]; len(fhs) > 0 {
			fh = fhs[0]
		} else {
			for _, fhs := range r.MultipartForm.File {
				if len(fhs) > 0 {
					fh = fhs[0]
					break
				}
			}
		}
		if fh == nil {
			return "", nil, errors.New("multipart: file part not found")
		}

		f, err := fh.Open()
		if err != nil {
			return "", nil, fmt.Errorf("multipart open: %w", err)
		}
		defer f.Close()
		reader = f
	}

	tmp, err := os.CreateTemp(os.TempDir(), "upload-*.bin")
	if err != nil {
		return "", nil, fmt.Errorf("tempfile: %w", err)
	}

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}

	n, err := io.Copy(tmp, reader)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("read body: %w", err)
	}
	if n == 0 {
		cleanup()
		return "", nil, errors.New("empty body")
	}

	_ = tmp.Sync()
	_ = tmp.Close()

	return filepath.Clean(tmp.Name()), cleanup, nil
}
