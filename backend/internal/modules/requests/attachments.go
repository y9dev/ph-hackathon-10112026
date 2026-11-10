package requests

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"modusvoice/internal/platform/apperr"
)

// AllowedExtensions is the upload allowlist (lowercase, no dot).
var AllowedExtensions = []string{"png", "jpg", "jpeg", "pdf", "docx", "xlsx", "txt"}

var extMIME = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"pdf":  "application/pdf",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"txt":  "text/plain; charset=utf-8",
}

// Upload is a file received from the client, not yet validated.
type Upload struct {
	Filename string
	Size     int64
	Open     func() (io.ReadCloser, error)
}

// storedFile is a validated file written to disk.
type storedFile struct {
	Filename   string
	StoredName string
	MIME       string
	Size       int64
}

// SanitizeFilename keeps a safe display name (no path, no control chars, <= 200 runes).
func SanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' || r == '<' || r == '>' || r == ':' || r == '|' || r == '?' || r == '*' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(strings.Trim(name, "."))
	if !utf8.ValidString(name) {
		name = strings.ToValidUTF8(name, "")
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[len(r)-200:])
	}
	if name == "" {
		name = "file"
	}
	return name
}

func extOf(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

// SniffMatches checks magic bytes of head against the declared extension.
func SniffMatches(ext string, head []byte) bool {
	switch ext {
	case "png":
		return bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n"))
	case "jpg", "jpeg":
		return bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF})
	case "pdf":
		return bytes.HasPrefix(head, []byte("%PDF-"))
	case "docx", "xlsx":
		return bytes.HasPrefix(head, []byte("PK\x03\x04"))
	case "txt":
		if bytes.IndexByte(head, 0) >= 0 {
			return false
		}
		// allow a multi-byte rune cut at the end of the sniff window
		for i := 0; i < 4 && len(head) > 0; i++ {
			if utf8.Valid(head) {
				return true
			}
			head = head[:len(head)-1]
		}
		return utf8.Valid(head)
	}
	return false
}

// ValidateUploads checks count, sizes and extensions before anything is written.
func ValidateUploads(files []Upload) error {
	if len(files) > MaxFiles {
		return apperr.Invalid("files", fmt.Sprintf("Можно прикрепить не более %d файлов", MaxFiles))
	}
	var total int64
	for _, f := range files {
		name := SanitizeFilename(f.Filename)
		ext := extOf(name)
		if _, ok := extMIME[ext]; !ok {
			return apperr.Invalid("files", fmt.Sprintf("Файл «%s»: недопустимый тип. Разрешены: %s", name, strings.Join(AllowedExtensions, ", ")))
		}
		if f.Size > MaxFileBytes {
			return apperr.Invalid("files", fmt.Sprintf("Файл «%s» больше 10 МБ", name))
		}
		if f.Size == 0 {
			return apperr.Invalid("files", fmt.Sprintf("Файл «%s» пустой", name))
		}
		total += f.Size
	}
	if total > MaxTotalBytes {
		return apperr.Invalid("files", "Общий размер файлов больше 10 МБ")
	}
	return nil
}

// storeUploads sniffs and writes files under dir with random names. On any error the files
// already written are removed.
func storeUploads(dir string, files []Upload) (out []storedFile, err error) {
	defer func() {
		if err != nil {
			removeStored(dir, out)
			out = nil
		}
	}()
	for _, f := range files {
		sf, err := storeOne(dir, f)
		if err != nil {
			return out, err
		}
		out = append(out, sf)
	}
	return out, nil
}

func storeOne(dir string, f Upload) (storedFile, error) {
	name := SanitizeFilename(f.Filename)
	ext := extOf(name)
	rc, err := f.Open()
	if err != nil {
		return storedFile{}, fmt.Errorf("open upload: %w", err)
	}
	defer rc.Close()
	head := make([]byte, 512)
	n, err := io.ReadFull(rc, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return storedFile{}, fmt.Errorf("read upload: %w", err)
	}
	head = head[:n]
	if !SniffMatches(ext, head) {
		return storedFile{}, apperr.Invalid("files", fmt.Sprintf("Файл «%s»: содержимое не соответствует расширению", name))
	}
	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return storedFile{}, fmt.Errorf("random name: %w", err)
	}
	stored := hex.EncodeToString(rnd)
	path := filepath.Join(dir, stored)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return storedFile{}, fmt.Errorf("create stored file: %w", err)
	}
	written, err := io.Copy(out, io.MultiReader(bytes.NewReader(head), io.LimitReader(rc, MaxFileBytes+1-int64(len(head)))))
	cerr := out.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
		return storedFile{}, fmt.Errorf("write stored file: %w", err)
	}
	if written > MaxFileBytes {
		_ = os.Remove(path)
		return storedFile{}, apperr.Invalid("files", fmt.Sprintf("Файл «%s» больше 10 МБ", name))
	}
	return storedFile{Filename: name, StoredName: stored, MIME: extMIME[ext], Size: written}, nil
}

func removeStored(dir string, files []storedFile) {
	for _, f := range files {
		_ = os.Remove(filepath.Join(dir, f.StoredName))
	}
}

// validStoredName guards against path traversal when serving files.
func validStoredName(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
