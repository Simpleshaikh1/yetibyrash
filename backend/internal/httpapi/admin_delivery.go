package httpapi

import (
	"net/http"

	"yetibyrash/backend/internal/delivery"
)

type adminDeliveryHandlers struct{ d *delivery.Service }

// GET /api/admin/v1/delivery
func (h adminDeliveryHandlers) get(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.Admin(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// PUT /api/admin/v1/delivery/zones/{id}
func (h adminDeliveryHandlers) updateZone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var z delivery.Zone
	if !decodeJSON(w, r, &z) {
		return
	}
	z.ID = id
	if writeAppError(w, h.d.UpdateZone(r.Context(), z)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/admin/v1/delivery/states/{id} {"zoneId"}
func (h adminDeliveryHandlers) setStateZone(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		ZoneID int64 `json:"zoneId"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.d.SetStateZone(r.Context(), id, in.ZoneID)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/v1/delivery/states/{id}/lgas — add an LGA to a state
func (h adminDeliveryHandlers) addLga(w http.ResponseWriter, r *http.Request) {
	stateID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var l delivery.Lga
	if !decodeJSON(w, r, &l) {
		return
	}
	id, err := h.d.SaveLga(r.Context(), 0, stateID, l)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// PUT /api/admin/v1/delivery/lgas/{id}
func (h adminDeliveryHandlers) updateLga(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var l delivery.Lga
	if !decodeJSON(w, r, &l) {
		return
	}
	if _, err := h.d.SaveLga(r.Context(), id, 0, l); writeAppError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/admin/v1/delivery/lgas/{id}
func (h adminDeliveryHandlers) deleteLga(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeAppError(w, h.d.DeleteLga(r.Context(), id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/admin/v1/delivery/settings
func (h adminDeliveryHandlers) updateSettings(w http.ResponseWriter, r *http.Request) {
	var s delivery.Settings
	if !decodeJSON(w, r, &s) {
		return
	}
	if writeAppError(w, h.d.UpdateSettings(r.Context(), s)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/v1/delivery/holidays {"day","name"}
func (h adminDeliveryHandlers) addHoliday(w http.ResponseWriter, r *http.Request) {
	var hol delivery.Holiday
	if !decodeJSON(w, r, &hol) {
		return
	}
	if writeAppError(w, h.d.AddHoliday(r.Context(), hol)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/admin/v1/delivery/holidays/{day}
func (h adminDeliveryHandlers) deleteHoliday(w http.ResponseWriter, r *http.Request) {
	if writeAppError(w, h.d.DeleteHoliday(r.Context(), r.PathValue("day"))) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
