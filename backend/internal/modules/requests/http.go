package requests

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"modusvoice/internal/platform/apperr"
	"modusvoice/internal/platform/authz"
	"modusvoice/internal/platform/httpx"
)

// MaxCreateBody bounds the multipart create request (10 MB of files + form overhead).
const MaxCreateBody = MaxTotalBytes + 1<<20

type handler struct {
	svc *Service
	loc *time.Location
}

func actor(r *http.Request) authz.Actor { a, _ := authz.FromContext(r.Context()); return a }

func (h *handler) meta(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.Meta(r.Context(), actor(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

func (h *handler) preview(w http.ResponseWriter, r *http.Request) {
	cat, err := httpx.QueryInt(r, "category_id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var id int64
	if cat != nil {
		id = *cat
	}
	p, err := h.svc.RoutingPreview(r.Context(), actor(r), id, r.URL.Query().Get("type"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func parseListQuery(r *http.Request) (ListQuery, error) {
	qs := r.URL.Query()
	q := ListQuery{Scope: qs.Get("scope"), Tab: qs.Get("tab"), Status: qs.Get("status"), Assignee: qs.Get("assignee"),
		Q: qs.Get("q"), Sort: qs.Get("sort"), Priority: qs.Get("priority"), RequestType: qs.Get("request_type")}
	var err error
	if q.CategoryID, err = httpx.QueryInt(r, "category_id"); err != nil {
		return q, err
	}
	if q.MineScope, err = httpx.QueryBool(r, "mine_scope"); err != nil {
		return q, err
	}
	if q.DepartmentID, err = httpx.QueryInt(r, "department_id"); err != nil {
		return q, err
	}
	if q.Overdue, err = httpx.QueryBool(r, "overdue"); err != nil {
		return q, err
	}
	page, err := httpx.QueryInt(r, "page")
	if err != nil {
		return q, err
	}
	size, err := httpx.QueryInt(r, "page_size")
	if err != nil {
		return q, err
	}
	if page != nil {
		q.Page = int(min(*page, 1_000_000))
	}
	if size != nil {
		q.PageSize = int(min(*size, 1000))
	}
	return q, nil
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	q, err := parseListQuery(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.List(r.Context(), actor(r), q)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *handler) export(w http.ResponseWriter, r *http.Request) {
	a := actor(r)
	if err := authz.RequireStaff(a); err != nil {
		httpx.Error(w, r, err)
		return
	}
	q, err := parseListQuery(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	// render into memory first so validation errors still produce the JSON envelope
	var buf strings.Builder
	if err := h.svc.ExportCSV(r.Context(), a, q, h.loc, &buf); err != nil {
		httpx.Error(w, r, err)
		return
	}
	name := "modus-voice-" + time.Now().In(h.loc).Format("2006-01-02") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(buf.String()))
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	var uploads []Upload
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch mt {
	case "multipart/form-data":
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				httpx.Error(w, r, apperr.Invalid("files", "Общий размер файлов больше 10 МБ"))
				return
			}
			httpx.Error(w, r, apperr.BadRequest("Некорректная форма"))
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()
		payload := r.MultipartForm.Value["payload"]
		if len(payload) != 1 {
			httpx.Error(w, r, apperr.Invalid("payload", "Поле payload обязательно"))
			return
		}
		if err := json.Unmarshal([]byte(payload[0]), &in); err != nil {
			httpx.Error(w, r, apperr.Invalid("payload", "Некорректный JSON в поле payload"))
			return
		}
		for _, fh := range r.MultipartForm.File["files"] {
			fh := fh
			uploads = append(uploads, Upload{Filename: fh.Filename, Size: fh.Size, Open: func() (io.ReadCloser, error) { return fh.Open() }})
		}
	case "application/json":
		if err := httpx.Decode(r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
	default:
		httpx.Error(w, r, apperr.BadRequest("Ожидается multipart/form-data или JSON"))
		return
	}
	if in.IsAnonymous && anonBlockedByCookie(r) {
		httpx.Error(w, r, spamBlockedError())
		return
	}
	d, err := h.svc.Create(r.Context(), actor(r), in, uploads)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if d.AnonymousToken != nil {
		rememberAnonToken(w, r, *d.AnonymousToken)
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.Get(r.Context(), actor(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in StatusInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.ChangeStatus(r.Context(), actor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) assign(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in struct {
		DepartmentID *int64           `json:"department_id"`
		AssigneeID   httpx.Opt[int64] `json:"assignee_id"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.Assign(r.Context(), actor(r), id, AssignInput{DepartmentID: in.DepartmentID, AssigneeSet: in.AssigneeID.Set, AssigneeID: in.AssigneeID.Ptr()})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) comment(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in CommentInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.AddComment(r.Context(), actor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *handler) theme(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in ThemeInput
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.LinkTheme(r.Context(), actor(r), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) attachment(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	at, err := h.svc.AttachmentFile(r.Context(), actor(r), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.serveFile(w, r, at)
}

func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, at AttachmentRow) {
	f, err := os.Open(filepath.Join(h.svc.UploadDir, at.StoredName))
	if err != nil {
		httpx.Error(w, r, apperr.NotFound())
		return
	}
	defer f.Close()
	ascii := strings.Map(func(c rune) rune {
		if c < 0x20 || c > 0x7e || c == '"' || c == '\\' {
			return '_'
		}
		return c
	}, at.Filename)
	w.Header().Set("Content-Type", at.MIME)
	w.Header().Set("Content-Disposition", `attachment; filename="`+ascii+`"; filename*=UTF-8''`+url.PathEscape(at.Filename))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, "", time.Time{}, f)
}

// clientIP uses the identity module's proxy-aware resolver when available (TRUSTED_PROXY), else the TCP peer.
func (h *handler) clientIP(r *http.Request) string {
	if p, ok := h.svc.People.(interface{ ClientIP(*http.Request) string }); ok {
		return p.ClientIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// --- anonymous author endpoints (CR2): no session, token in the body / X-Anonymous-Token ---

func (h *handler) anonLookup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.AnonymousLookup(r.Context(), h.clientIP(r), in.Token)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) anonComment(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
		Body  string `json:"body"`
	}
	if err := httpx.Decode(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.AnonymousComment(r.Context(), h.clientIP(r), in.Token, in.Body)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *handler) anonAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.IDParam(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	at, err := h.svc.AnonymousAttachment(r.Context(), h.clientIP(r), r.Header.Get("X-Anonymous-Token"), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.serveFile(w, r, at)
}

func mountRoutes(h *handler, pub, priv chi.Router) {
	pub.Post("/anonymous/lookup", h.anonLookup)
	pub.Post("/anonymous/comments", h.anonComment)
	pub.Get("/anonymous/attachments/{id}", h.anonAttachment)
	pub.Post("/anonymous/status", h.anonStatus)
	priv.Get("/meta", h.meta)
	priv.Get("/routing/preview", h.preview)
	priv.Get("/requests", h.list)
	priv.Post("/requests", h.create)
	priv.Get("/requests/export.csv", h.export)
	priv.Get("/requests/{id}", h.get)
	priv.Post("/requests/{id}/status", h.status)
	priv.Post("/requests/{id}/assign", h.assign)
	priv.Post("/requests/{id}/spam", h.markSpam)
	priv.Post("/requests/{id}/comments", h.comment)
	priv.Post("/requests/{id}/theme", h.theme)
	priv.Get("/attachments/{id}", h.attachment)
}
