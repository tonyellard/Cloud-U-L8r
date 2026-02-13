package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/tonyellard/kay-vee/internal/model"
	"github.com/tonyellard/kay-vee/internal/storage"
)

type Server struct {
	logger *slog.Logger
	store  *storage.Store
}

func NewRouter(logger *slog.Logger) http.Handler {
	srv := &Server{logger: logger, store: storage.NewStore("us-east-1", "000000000000")}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/", srv.handleAWSJSON)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "kay-vee"})
}

func (s *Server) handleAWSJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "X-Amz-Target header is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "failed to read request body")
		return
	}

	switch target {
	case "AmazonSSM.PutParameter":
		s.handlePutParameter(w, body)
	case "AmazonSSM.GetParameter":
		s.handleGetParameter(w, body)
	case "AmazonSSM.GetParameters":
		s.handleGetParameters(w, body)
	case "secretsmanager.CreateSecret":
		s.handleCreateSecret(w, body)
	case "secretsmanager.GetSecretValue":
		s.handleGetSecretValue(w, body)
	case "secretsmanager.PutSecretValue":
		s.handlePutSecretValue(w, body)
	case "secretsmanager.UpdateSecret":
		s.handleUpdateSecret(w, body)
	default:
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "unsupported target: "+target)
	}
}

func (s *Server) handlePutParameter(w http.ResponseWriter, body []byte) {
	var req model.PutParameterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.PutParameter(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetParameter(w http.ResponseWriter, body []byte) {
	var req model.GetParameterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	param, err := s.store.GetParameter(req.Name, req.WithDecryption)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.GetParameterResponse{Parameter: param})
}

func (s *Server) handleGetParameters(w http.ResponseWriter, body []byte) {
	var req model.GetParametersRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	params, invalid := s.store.GetParameters(req.Names, req.WithDecryption)
	writeJSON(w, http.StatusOK, model.GetParametersResponse{Parameters: params, InvalidParameters: invalid})
}

func (s *Server) handleCreateSecret(w http.ResponseWriter, body []byte) {
	var req model.CreateSecretRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.CreateSecret(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetSecretValue(w http.ResponseWriter, body []byte) {
	var req model.GetSecretValueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.GetSecretValue(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handlePutSecretValue(w http.ResponseWriter, body []byte) {
	var req model.PutSecretValueRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.PutSecretValue(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUpdateSecret(w http.ResponseWriter, body []byte) {
	var req model.UpdateSecretRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.UpdateSecret(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAWSError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, model.AWSJSONError{Type: code, Message: message})
}

func writeFromError(w http.ResponseWriter, err error) {
	if err == nil {
		writeAWSError(w, http.StatusInternalServerError, "InternalFailure", "unknown error")
		return
	}
	msg := err.Error()
	typeName := "InternalFailure"
	status := http.StatusInternalServerError

	if strings.Contains(msg, ":") {
		parts := strings.SplitN(msg, ":", 2)
		typeName = strings.TrimSpace(parts[0])
		msg = strings.TrimSpace(parts[1])
	}

	switch typeName {
	case "ValidationException", "ParameterAlreadyExists", "InvalidParameterException", "InvalidRequestException", "ResourceExistsException":
		status = http.StatusBadRequest
	case "ParameterNotFound", "ResourceNotFoundException":
		status = http.StatusNotFound
	}

	writeAWSError(w, status, typeName, msg)
}

func IsValidation(err error) bool {
	return errors.Is(err, errors.New("validation"))
}
