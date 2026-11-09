package handler

import (
	"io"
	"net/http"
	"os"

	"kutubcore/internal/domain/files"
	"kutubcore/internal/middleware"
	"kutubcore/internal/storage"
)

// GET /api/v1/books/{id}/files — public metadata only (Blueprint §9).
func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	list, err := h.Files.ListFiles(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list files.")
		return
	}
	if list == nil {
		list = []files.File{}
	}
	writeJSON(w, http.StatusOK, list, nil)
}

// POST /api/v1/books/{id}/files — organizer.
// multipart/form-data: field "file" (PDF/EPUB upload) OR JSON {"external_url": "..."}.
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	bookID := r.PathValue("id")
	ct := r.Header.Get("Content-Type")

	// JSON external URL.
	if len(ct) >= 16 && ct[:16] == "application/json" {
		var body struct {
			ExternalURL string `json:"external_url"`
			Format      string `json:"format"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
			return
		}
		f, err := h.FileSvc.AddExternalLocation(r.Context(), u.ID, bookID, body.ExternalURL, body.Format)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, f, nil)
		return
	}

	// Multipart file upload.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid multipart form.")
		return
	}
	fh, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Missing 'file' field.")
		return
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, 60<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Could not read file.")
		return
	}
	filename := ""
	if r.MultipartForm != nil {
		if h2, ok := r.MultipartForm.File["file"]; ok && len(h2) > 0 {
			filename = h2[0].Filename
		}
	}
	created, err := h.FileSvc.UploadBookFile(r.Context(), u.ID, bookID, filename, r.FormValue("format"), data)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// POST /api/v1/books/{id}/cover — organizer (multipart image).
func (h *Handler) UploadCover(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid multipart form.")
		return
	}
	fh, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Missing 'file' field.")
		return
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, 6<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Could not read file.")
		return
	}
	filename := ""
	if r.MultipartForm != nil {
		if h2, ok := r.MultipartForm.File["file"]; ok && len(h2) > 0 {
			filename = h2[0].Filename
		}
	}
	created, err := h.FileSvc.UploadCover(r.Context(), u.ID, r.PathValue("id"), filename, data)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// GET /api/v1/files/{id}/download — authorized (serves local file or
// redirects to external URL). Never leaks private user data; file metadata
// itself is catalog-public.
func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	f, err := h.Files.GetFile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	for _, loc := range f.Locations {
		if loc.LocationType == "local" && loc.IsAvailable {
			path, err := storage.Open(h.FileDir, loc.StoragePath)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", "File path invalid.")
				return
			}
			info, err := os.Stat(path)
			if err != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "File bytes not found.")
				return
			}
			fh, err := os.Open(path)
			if err != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "File bytes not found.")
				return
			}
			defer fh.Close()
			mime := f.MimeType
			if mime == "" {
				mime = "application/octet-stream"
			}
			w.Header().Set("Content-Type", mime)
			w.Header().Set("Content-Disposition", `attachment; filename="`+f.FileName+`"`)
			http.ServeContent(w, r, f.FileName, info.ModTime(), fh)
			return
		}
		if loc.LocationType == "external_url" && loc.ExternalURL != "" {
			http.Redirect(w, r, loc.ExternalURL, http.StatusFound)
			return
		}
	}
	// A saved URL with IsAvailable=false is not downloadable (rule 10);
	// redirect to the external URL so users can fetch it themselves.
	for _, loc := range f.Locations {
		if loc.ExternalURL != "" {
			http.Redirect(w, r, loc.ExternalURL, http.StatusFound)
			return
		}
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "No available location for this file.")
}
