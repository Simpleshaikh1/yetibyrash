package httpapi

import (
	"errors"
	"net/http"

	"yetibyrash/backend/internal/content"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/promotions"
)

// ---------- Shop ----------

type siteHandlers struct{ c *content.Service }

// GET /api/v1/site — announcement bar and footer pages
func (h siteHandlers) site(w http.ResponseWriter, r *http.Request) {
	s, err := h.c.Site(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// GET /api/v1/pages/{slug}
func (h siteHandlers) page(w http.ResponseWriter, r *http.Request) {
	p, err := h.c.Page(r.Context(), r.PathValue("slug"))
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// POST /api/v1/cart/code {"code"}
func (h *shopHandlers) applyCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	id := h.cartID(r)
	if writeAppError(w, h.cart.ApplyCode(r.Context(), id, in.Code)) {
		return
	}
	h.writeBag(w, r, id)
}

// DELETE /api/v1/cart/code
func (h *shopHandlers) removeCode(w http.ResponseWriter, r *http.Request) {
	id := h.cartID(r)
	if writeAppError(w, h.cart.RemoveCode(r.Context(), id)) {
		return
	}
	h.writeBag(w, r, id)
}

// ---------- Admin: content ----------

type adminContentHandlers struct{ c *content.Service }

func (h adminContentHandlers) get(w http.ResponseWriter, r *http.Request) {
	v, err := h.c.Admin(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h adminContentHandlers) saveBanner(w http.ResponseWriter, r *http.Request) {
	var b content.Banner
	if !decodeJSON(w, r, &b) || !idFromPath(w, r, &b.ID) {
		return
	}
	id, err := h.c.SaveBanner(r.Context(), currentMember(r), b)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

// POST /api/admin/v1/content/banners/{id}/image — multipart form: photo (file)
func (h adminContentHandlers) bannerImage(w http.ResponseWriter, r *http.Request) {
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
	photo, err := media.ProcessBanner(file)
	if errors.Is(err, media.ErrNotAnImage) {
		writeError(w, http.StatusBadRequest, "not_an_image", "That file isn't a photo. Upload a JPEG, PNG or WebP.")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_photo", "That photo couldn't be read. Try exporting it again as a JPEG.")
		return
	}
	key, err := h.c.SetBannerImage(r.Context(), currentMember(r), id, photo)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"image": key, "warnings": photo.Warnings})
}

func (h adminContentHandlers) deleteBanner(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeAppError(w, h.c.DeleteBanner(r.Context(), currentMember(r), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminContentHandlers) orderBanners(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.c.OrderBanners(r.Context(), currentMember(r), in.IDs)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminContentHandlers) saveAnnouncement(w http.ResponseWriter, r *http.Request) {
	var a content.Announcement
	if !decodeJSON(w, r, &a) || !idFromPath(w, r, &a.ID) {
		return
	}
	id, err := h.c.SaveAnnouncement(r.Context(), currentMember(r), a)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (h adminContentHandlers) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeAppError(w, h.c.DeleteAnnouncement(r.Context(), currentMember(r), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminContentHandlers) orderAnnouncements(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.c.OrderAnnouncements(r.Context(), currentMember(r), in.IDs)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminContentHandlers) saveSections(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Sections []content.Section `json:"sections"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.c.SaveSections(r.Context(), currentMember(r), in.Sections)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminContentHandlers) savePage(w http.ResponseWriter, r *http.Request) {
	var p content.Page
	if !decodeJSON(w, r, &p) || !idFromPath(w, r, &p.ID) {
		return
	}
	id, err := h.c.SavePage(r.Context(), currentMember(r), p)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (h adminContentHandlers) deletePage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeAppError(w, h.c.DeletePage(r.Context(), currentMember(r), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Admin: promotions ----------

type adminPromoHandlers struct{ p *promotions.Service }

func (h adminPromoHandlers) get(w http.ResponseWriter, r *http.Request) {
	v, err := h.p.Admin(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h adminPromoHandlers) saveSale(w http.ResponseWriter, r *http.Request) {
	var a promotions.AdminSale
	if !decodeJSON(w, r, &a) || !idFromPath(w, r, &a.ID) {
		return
	}
	id, err := h.p.SaveSale(r.Context(), currentMember(r), a)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (h adminPromoHandlers) deleteSale(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeAppError(w, h.p.DeleteSale(r.Context(), currentMember(r), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h adminPromoHandlers) saveCode(w http.ResponseWriter, r *http.Request) {
	var c promotions.AdminCode
	if !decodeJSON(w, r, &c) || !idFromPath(w, r, &c.ID) {
		return
	}
	id, err := h.p.SaveCode(r.Context(), currentMember(r), c)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

func (h adminPromoHandlers) saveGift(w http.ResponseWriter, r *http.Request) {
	var g promotions.AdminGift
	if !decodeJSON(w, r, &g) || !idFromPath(w, r, &g.ID) {
		return
	}
	id, err := h.p.SaveGift(r.Context(), currentMember(r), g)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": id})
}

// idFromPath sets *id from {id} on PUT routes and leaves it 0 on POST (create) routes.
func idFromPath(w http.ResponseWriter, r *http.Request, id *int64) bool {
	*id = 0
	if r.PathValue("id") == "" {
		return true
	}
	v, ok := pathID(w, r, "id")
	*id = v
	return ok
}
