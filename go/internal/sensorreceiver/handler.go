// Package sensorreceiver implements a narrow mTLS-only sensor batch custody HTTP boundary.
// It does not write the observation store or acknowledge complete ingestion.
package sensorreceiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

const BatchPath = "/v1/sensor/batches"

// IdentityPolicy maps an explicitly trusted client URI SAN to an authorized
// logical sensor identity. The TLS listener is required to verify certificates
// against a dedicated sensor trust root before invoking this handler.
type IdentityPolicy map[string]string

type Handler struct {
	custody    *sensoringest.Custody
	identities IdentityPolicy
	slots      chan struct{}
}

func NewHandler(custody *sensoringest.Custody, identities IdentityPolicy) (*Handler, error) {
	if custody == nil { return nil, fmt.Errorf("custody required") }
	if len(identities) == 0 { return nil, fmt.Errorf("at least one sensor identity is required") }
	copyMap := make(IdentityPolicy, len(identities))
	for uri, id := range identities {
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Scheme != "spiffe" || parsed.Host == "" || parsed.Path == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid authorized sensor URI SAN")
		}
		if strings.TrimSpace(id) == "" || strings.ContainsAny(id, " \t\r\n") { return nil, fmt.Errorf("invalid configured sensor identity") }
		copyMap[uri] = id
	}
	return &Handler{custody: custody, identities: copyMap, slots: make(chan struct{}, 8)}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != BatchPath { http.NotFound(w,r); return }
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		replyError(w,http.StatusMethodNotAllowed,"method_not_allowed")
		return
	}
	sensorID, ok := h.authenticatedSensor(r)
	if !ok { replyError(w,http.StatusUnauthorized,"unauthorized"); return }
	if r.Header.Get("X-Pathfinder-Sensor-ID") != sensorID {
		replyError(w,http.StatusForbidden,"sensor_identity_mismatch")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func(){ <- h.slots }()
	default:
		replyError(w,http.StatusServiceUnavailable,"receiver_busy")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-ndjson" {
		replyError(w,http.StatusUnsupportedMediaType,"unsupported_media_type")
		return
	}
	if r.ContentLength > sensoringest.MaxBatchBytes {
		replyError(w,http.StatusRequestEntityTooLarge,"batch_too_large")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(sensoringest.MaxBatchBytes)+1))
	if err != nil { replyError(w,http.StatusBadRequest,"body_read_failed"); return }
	if len(body) > sensoringest.MaxBatchBytes {
		replyError(w,http.StatusRequestEntityTooLarge,"batch_too_large")
		return
	}
	batch := sensoringest.Batch{
		SensorID:sensorID,
		Stream:sensoringest.Stream(r.Header.Get("X-Pathfinder-Stream")),
		SchemaVersion:r.Header.Get("X-Pathfinder-Schema-Version"),
		BatchID:r.Header.Get("X-Pathfinder-Batch-ID"),
		Payload:body,
	}
	receipt, err := h.custody.Preserve(batch)
	if err != nil {
		switch {
		case errors.Is(err,sensoringest.ErrUnsupported):
			replyError(w,http.StatusUnprocessableEntity,"unsupported_schema")
		case errors.Is(err,sensoringest.ErrMalformed), errors.Is(err,sensoringest.ErrSensorMismatch):
			replyError(w,http.StatusUnprocessableEntity,"malformed_batch")
		case errors.Is(err,sensoringest.ErrInvalidBatch):
			replyError(w,http.StatusUnprocessableEntity,"invalid_batch")
		case errors.Is(err,sensoringest.ErrBatchConflict):
			replyError(w,http.StatusConflict,"batch_conflict")
		case errors.Is(err,sensoringest.ErrCustodyCorrupt):
			replyError(w,http.StatusServiceUnavailable,"custody_unavailable")
		default:
			// Never echo untrusted records or filesystem paths.
			replyError(w,http.StatusServiceUnavailable,"custody_unavailable")
		}
		return
	}
	w.Header().Set("Content-Type","application/json")
	w.Header().Set("Cache-Control","no-store")
	status:=http.StatusCreated
	result:="preserved"
	if receipt.AlreadyPreserved { status=http.StatusOK; result="already_preserved" }
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
		Receipt sensoringest.CustodyReceipt `json:"receipt"`
		Indexed bool `json:"indexed"`
	}{Status:result,Receipt:receipt,Indexed:false})
}

func (h *Handler) authenticatedSensor(r *http.Request) (string,bool) {
	if r.TLS == nil || !r.TLS.HandshakeComplete || len(r.TLS.VerifiedChains)==0 || len(r.TLS.PeerCertificates)==0 { return "",false }
	leaf:=r.TLS.PeerCertificates[0]
	matched:=""
	for _,san:=range leaf.URIs {
		id,allowed:=h.identities[san.String()]
		if !allowed { continue }
		if matched!="" && matched!=id { return "",false }
		matched=id
	}
	return matched,matched!=""
}

func replyError(w http.ResponseWriter,status int,code string) {
	w.Header().Set("Content-Type","application/json")
	w.Header().Set("Cache-Control","no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status":"rejected","code":code})
}
