package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/staff"
)

// AdminCatalog is what the product editor and Catalogue setup need. *catalog.AdminService satisfies it.
type AdminCatalog interface {
	ListProducts(ctx context.Context, f catalog.AdminProductFilter) ([]catalog.AdminProductRow, error)
	GetProduct(ctx context.Context, id int64) (catalog.AdminProduct, error)
	CreateProduct(ctx context.Context, by staff.Member, in catalog.ProductInput) (int64, error)
	UpdateProduct(ctx context.Context, by staff.Member, id int64, in catalog.ProductInput) error
	AddImage(ctx context.Context, by staff.Member, productID int64, colourID *int64, kind string, photo *media.Processed) (catalog.AdminImage, error)
	DeleteImage(ctx context.Context, by staff.Member, productID, imageID int64) error
	MakeImageMain(ctx context.Context, by staff.Member, productID, imageID int64) error
	Setup(ctx context.Context) (catalog.Setup, error)
	SaveSizeSet(ctx context.Context, by staff.Member, id int64, in catalog.SizeSetInput) (int64, error)
	SaveColour(ctx context.Context, by staff.Member, id int64, in catalog.ColourInput) (int64, error)
	DeleteColour(ctx context.Context, by staff.Member, id int64) error
	SaveCategory(ctx context.Context, by staff.Member, id int64, in catalog.CategoryInput) (int64, error)
	SaveCategoryFields(ctx context.Context, by staff.Member, categoryID int64, fields []catalog.FieldInput) error
}

type adminCatalogHandlers struct{ c AdminCatalog }

// writeCatalogError turns a catalogue error into a response. Returns true if there was an error.
func writeCatalogError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var verr *catalog.ValidationError
	switch {
	case errors.As(err, &verr):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{
			"code": "invalid", "message": verr.Message, "field": verr.Field,
		}})
	case errors.Is(err, catalog.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "That item doesn't exist any more. Reload the page.")
	case errors.Is(err, catalog.ErrInUse):
		msg := strings.TrimPrefix(err.Error(), catalog.ErrInUse.Error()+": ")
		if strings.Contains(msg, "_fkey") {
			msg = "Products still use this, so it can't be removed."
		}
		writeError(w, http.StatusConflict, "in_use", msg)
	case errors.Is(err, catalog.ErrConflict):
		writeError(w, http.StatusConflict, "already_exists", "That name or SKU is already used. Choose another.")
	default:
		internalError(w, "admin catalogue", err)
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "That item doesn't exist.")
		return 0, false
	}
	return id, true
}

// GET /api/admin/v1/products?search=&status=&category=
func (h adminCatalogHandlers) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := catalog.AdminProductFilter{Search: q.Get("search"), Status: catalog.ProductStatus(q.Get("status"))}
	switch f.Status {
	case "", catalog.StatusDraft, catalog.StatusVisible, catalog.StatusHidden:
	default:
		writeError(w, http.StatusBadRequest, "invalid_status", "Status must be draft, visible or hidden.")
		return
	}
	if c := q.Get("category"); c != "" {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_category", "Category must be a category id.")
			return
		}
		f.CategoryID = id
	}
	rows, err := h.c.ListProducts(r.Context(), f)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": rows})
}

func (h adminCatalogHandlers) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p, err := h.c.GetProduct(r.Context(), id)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h adminCatalogHandlers) createProduct(w http.ResponseWriter, r *http.Request) {
	var in catalog.ProductInput
	if !decodeJSON(w, r, &in) {
		return
	}
	id, err := h.c.CreateProduct(r.Context(), currentMember(r), in)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h adminCatalogHandlers) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in catalog.ProductInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeCatalogError(w, h.c.UpdateProduct(r.Context(), currentMember(r), id, in)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/v1/products/{id}/images — multipart form: photo (file), colourId, kind (studio|real_life)
func (h adminCatalogHandlers) uploadImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "That photo is too big. Photos can be up to 15 MB.")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no_photo", "Choose a photo to upload.")
		return
	}
	defer file.Close()

	var colourID *int64
	if c := r.FormValue("colourId"); c != "" {
		v, err := strconv.ParseInt(c, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_colour", "Colour must be a colour id.")
			return
		}
		colourID = &v
	}

	photo, err := media.Process(file)
	if errors.Is(err, media.ErrNotAnImage) {
		writeError(w, http.StatusBadRequest, "not_an_image", "That file isn't a photo. Upload a JPEG, PNG or WebP.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_photo", "That photo couldn't be read. Try exporting it again as a JPEG.")
		return
	}
	img, err := h.c.AddImage(r.Context(), currentMember(r), id, colourID, r.FormValue("kind"), photo)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, img)
}

func (h adminCatalogHandlers) deleteImage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	imageID, ok := pathID(w, r, "imageId")
	if !ok {
		return
	}
	if writeCatalogError(w, h.c.DeleteImage(r.Context(), currentMember(r), id, imageID)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminCatalogHandlers) makeImageMain(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	imageID, ok := pathID(w, r, "imageId")
	if !ok {
		return
	}
	if writeCatalogError(w, h.c.MakeImageMain(r.Context(), currentMember(r), id, imageID)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminCatalogHandlers) setup(w http.ResponseWriter, r *http.Request) {
	s, err := h.c.Setup(r.Context())
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// optionalID reads {id} from the path; routes without it (POST to create) give 0.
func optionalID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if r.PathValue("id") == "" {
		return 0, true
	}
	return pathID(w, r, "id")
}

func (h adminCatalogHandlers) saveSizeSet(w http.ResponseWriter, r *http.Request) {
	id, ok := optionalID(w, r)
	if !ok {
		return
	}
	var in catalog.SizeSetInput
	if !decodeJSON(w, r, &in) {
		return
	}
	newID, err := h.c.SaveSizeSet(r.Context(), currentMember(r), id, in)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": newID})
}

func (h adminCatalogHandlers) saveColour(w http.ResponseWriter, r *http.Request) {
	id, ok := optionalID(w, r)
	if !ok {
		return
	}
	var in catalog.ColourInput
	if !decodeJSON(w, r, &in) {
		return
	}
	newID, err := h.c.SaveColour(r.Context(), currentMember(r), id, in)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": newID})
}

func (h adminCatalogHandlers) deleteColour(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeCatalogError(w, h.c.DeleteColour(r.Context(), currentMember(r), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminCatalogHandlers) saveCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := optionalID(w, r)
	if !ok {
		return
	}
	var in catalog.CategoryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	newID, err := h.c.SaveCategory(r.Context(), currentMember(r), id, in)
	if writeCatalogError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": newID})
}

func (h adminCatalogHandlers) saveCategoryFields(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Fields []catalog.FieldInput `json:"fields"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeCatalogError(w, h.c.SaveCategoryFields(r.Context(), currentMember(r), id, in.Fields)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
