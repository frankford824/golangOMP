package office

import (
	"database/sql"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"workflow/internal/oa/platform"
)

func templateFallback() string { return os.Getenv("OA_FALLBACK_APPROVER_ID") }

func (app office) upload(w http.ResponseWriter, r *http.Request) {
	const limit = 10 << 20
	dir := os.Getenv("OA_FILES_DIR")
	if dir == "" {
		platform.Error(w, 503, "费用附件存储尚未配置")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit+65536)
	reader, err := r.MultipartReader()
	if err != nil {
		bad(w, "请选择附件")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || !textOK(part.FileName(), 255) {
		bad(w, "请选择附件")
		return
	}
	defer part.Close()
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil || len(data) == 0 || len(data) > limit {
		bad(w, "附件大小须在 10 MB 内")
		return
	}
	kind := http.DetectContentType(data)
	switch kind {
	case "application/pdf", "image/png", "image/jpeg":
	default:
		bad(w, "只接受 PDF、PNG 或 JPEG")
		return
	}
	if os.MkdirAll(dir, 0700) != nil {
		internal(w)
		return
	}
	id := identifier()
	path := filepath.Join(dir, id)
	if os.WriteFile(path, data, 0600) != nil {
		internal(w)
		return
	}
	_, err = app.db.ExecContext(r.Context(), "INSERT INTO oa_office_files(id,owner_id,name,mime) VALUES(?,?,?,?)", id, platform.ActorFrom(r).ID, part.FileName(), kind)
	if err != nil {
		_ = os.Remove(path)
		internal(w)
		return
	}
	platform.JSON(w, 201, map[string]string{"id": id, "name": part.FileName()})
}
func linkFiles(r *http.Request, tx *sql.Tx, files []string, owner int64, documentID string) bool {
	for _, id := range files {
		if len(id) != 32 {
			return false
		}
		if _, err := hex.DecodeString(id); err != nil {
			return false
		}
		var linked sql.NullString
		var actualOwner int64
		if tx.QueryRowContext(r.Context(), "SELECT owner_id,document_id FROM oa_office_files WHERE id=? FOR UPDATE", id).Scan(&actualOwner, &linked) != nil || actualOwner != owner || linked.Valid && linked.String != documentID {
			return false
		}
		if !linked.Valid {
			if _, err := tx.ExecContext(r.Context(), "UPDATE oa_office_files SET document_id=? WHERE id=?", documentID, id); err != nil {
				return false
			}
		}
	}
	return true
}
func (app office) download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if len(id) != 32 {
		missing(w)
		return
	}
	if _, err := hex.DecodeString(id); err != nil {
		missing(w)
		return
	}
	dir := os.Getenv("OA_FILES_DIR")
	if dir == "" {
		missing(w)
		return
	}
	var owner int64
	var name, kind string
	var linked sql.NullString
	if err := app.db.QueryRowContext(r.Context(), "SELECT owner_id,name,mime,document_id FROM oa_office_files WHERE id=?", id).Scan(&owner, &name, &kind, &linked); err != nil {
		missing(w)
		return
	}
	actor := platform.ActorFrom(r)
	if actor.ID != owner {
		if !linked.Valid {
			deny(w)
			return
		}
		doc, err := scanDocument(app.db.QueryRowContext(r.Context(), "SELECT payload FROM oa_office_documents WHERE id=?", linked.String))
		if err != nil || !doc.canRead(actor) {
			deny(w)
			return
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	http.ServeFile(w, r, filepath.Join(dir, id))
}
