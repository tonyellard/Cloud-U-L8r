package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
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
	mux.HandleFunc("/admin/api/summary", srv.handleAdminSummary)
	mux.HandleFunc("/admin/api/resources", srv.handleAdminResources)
	mux.HandleFunc("/admin/api/activity", srv.handleAdminActivity)
	mux.HandleFunc("/admin/api/export", srv.handleAdminExport)
	mux.HandleFunc("/admin/api/import", srv.handleAdminImport)
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
	target := r.Header.Get("X-Amz-Target")
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		s.store.RecordActivity(model.AdminActivityEntry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Target:     target,
			StatusCode: recorder.status,
			ErrorType:  parseErrorType(recorder.body.Bytes()),
		})
	}()

	if r.Method != http.MethodPost {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if target == "" {
		writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "X-Amz-Target header is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "failed to read request body")
		return
	}

	switch target {
	case "AmazonSSM.PutParameter":
		s.handlePutParameter(recorder, body)
	case "AmazonSSM.LabelParameterVersion":
		s.handleLabelParameterVersion(recorder, body)
	case "AmazonSSM.DescribeParameters":
		s.handleDescribeParameters(recorder, body)
	case "AmazonSSM.DeleteParameter":
		s.handleDeleteParameter(recorder, body)
	case "AmazonSSM.DeleteParameters":
		s.handleDeleteParameters(recorder, body)
	case "AmazonSSM.GetParameter":
		s.handleGetParameter(recorder, body)
	case "AmazonSSM.GetParameterHistory":
		s.handleGetParameterHistory(recorder, body)
	case "AmazonSSM.GetParameters":
		s.handleGetParameters(recorder, body)
	case "AmazonSSM.GetParametersByPath":
		s.handleGetParametersByPath(recorder, body)
	case "secretsmanager.CreateSecret":
		s.handleCreateSecret(recorder, body)
	case "secretsmanager.GetSecretValue":
		s.handleGetSecretValue(recorder, body)
	case "secretsmanager.PutSecretValue":
		s.handlePutSecretValue(recorder, body)
	case "secretsmanager.UpdateSecret":
		s.handleUpdateSecret(recorder, body)
	case "secretsmanager.DescribeSecret":
		s.handleDescribeSecret(recorder, body)
	case "secretsmanager.ListSecrets":
		s.handleListSecrets(recorder, body)
	case "secretsmanager.DeleteSecret":
		s.handleDeleteSecret(recorder, body)
	case "secretsmanager.RestoreSecret":
		s.handleRestoreSecret(recorder, body)
	case "secretsmanager.UpdateSecretVersionStage":
		s.handleUpdateSecretVersionStage(recorder, body)
	default:
		writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "unsupported target: "+target)
	}
}

func (s *Server) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		s.store.RecordActivity(model.AdminActivityEntry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Target:     "admin.summary",
			StatusCode: recorder.status,
			ErrorType:  parseErrorType(recorder.body.Bytes()),
		})
	}()

	if r.Method != http.MethodGet {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(recorder, http.StatusOK, s.store.Summary())
}

func (s *Server) handleAdminActivity(w http.ResponseWriter, r *http.Request) {
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	if r.Method != http.MethodGet {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	maxResults := 0
	if maxStr := r.URL.Query().Get("maxResults"); maxStr != "" {
		parsed, err := strconv.Atoi(maxStr)
		if err != nil {
			writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid maxResults query parameter")
			return
		}
		maxResults = parsed
	}

	entries, token, err := s.store.ListActivity(maxResults, r.URL.Query().Get("nextToken"))
	if err != nil {
		writeFromError(recorder, err)
		return
	}
	writeJSON(recorder, http.StatusOK, model.AdminActivityResponse{Activity: entries, NextToken: token})
}

func (s *Server) handleAdminResources(w http.ResponseWriter, r *http.Request) {
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	if r.Method != http.MethodGet {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	includeParameters := true
	if raw := strings.TrimSpace(r.URL.Query().Get("includeParameters")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid includeParameters query parameter")
			return
		}
		includeParameters = parsed
	}

	includeSecrets := true
	if raw := strings.TrimSpace(r.URL.Query().Get("includeSecrets")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid includeSecrets query parameter")
			return
		}
		includeSecrets = parsed
	}

	res := model.AdminResourcesResponse{}

	if includeParameters {
		parameterPath := strings.TrimSpace(r.URL.Query().Get("parameterPath"))
		if parameterPath == "" {
			parameterPath = "/"
		}

		recursive := true
		if raw := strings.TrimSpace(r.URL.Query().Get("recursive")); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid recursive query parameter")
				return
			}
			recursive = parsed
		}

		withDecryption := false
		if raw := strings.TrimSpace(r.URL.Query().Get("withDecryption")); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid withDecryption query parameter")
				return
			}
			withDecryption = parsed
		}

		parameterMaxResults := 10
		if raw := strings.TrimSpace(r.URL.Query().Get("parameterMaxResults")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid parameterMaxResults query parameter")
				return
			}
			parameterMaxResults = parsed
		}

		parameterFilters := make([]model.ParameterStringFilter, 0, 2)
		if value := strings.TrimSpace(r.URL.Query().Get("parameterType")); value != "" {
			parameterFilters = append(parameterFilters, model.ParameterStringFilter{Key: "Type", Option: "Equals", Values: []string{value}})
		}
		if value := strings.TrimSpace(r.URL.Query().Get("parameterLabel")); value != "" {
			parameterFilters = append(parameterFilters, model.ParameterStringFilter{Key: "Label", Option: "Equals", Values: []string{value}})
		}

		params, nextToken, err := s.store.GetParametersByPath(parameterPath, recursive, withDecryption, parameterMaxResults, strings.TrimSpace(r.URL.Query().Get("parametersNextToken")), parameterFilters)
		if err != nil {
			writeFromError(recorder, err)
			return
		}

		res.Parameters = params
		res.ParametersNextToken = nextToken
	}

	if includeSecrets {
		secretMaxResults := 25
		if raw := strings.TrimSpace(r.URL.Query().Get("secretMaxResults")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid secretMaxResults query parameter")
				return
			}
			secretMaxResults = parsed
		}

		secretFilters := make([]model.SecretFilter, 0, 1)
		if nameFilter := strings.TrimSpace(r.URL.Query().Get("secretName")); nameFilter != "" {
			secretFilters = append(secretFilters, model.SecretFilter{Key: "name", Values: []string{nameFilter}})
		}

		secrets, err := s.store.ListSecrets(secretMaxResults, strings.TrimSpace(r.URL.Query().Get("secretsNextToken")), secretFilters)
		if err != nil {
			writeFromError(recorder, err)
			return
		}

		res.Secrets = secrets.SecretList
		res.SecretsNextToken = secrets.NextToken
	}

	writeJSON(recorder, http.StatusOK, res)
}

func (s *Server) handleAdminExport(w http.ResponseWriter, r *http.Request) {
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		s.store.RecordActivity(model.AdminActivityEntry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Target:     "admin.export",
			StatusCode: recorder.status,
			ErrorType:  parseErrorType(recorder.body.Bytes()),
		})
	}()

	if r.Method != http.MethodGet {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(recorder, http.StatusOK, s.store.ExportState())
}

func (s *Server) handleAdminImport(w http.ResponseWriter, r *http.Request) {
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		s.store.RecordActivity(model.AdminActivityEntry{
			Method:     r.Method,
			Path:       r.URL.Path,
			Target:     "admin.import",
			StatusCode: recorder.status,
			ErrorType:  parseErrorType(recorder.body.Bytes()),
		})
	}()

	if r.Method != http.MethodPost {
		recorder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "failed to read request body")
		return
	}

	var req model.AdminImportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(recorder, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res := s.store.ImportState(req)
	writeJSON(recorder, http.StatusOK, res)
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

func (s *Server) handleLabelParameterVersion(w http.ResponseWriter, body []byte) {
	var req model.LabelParameterVersionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.LabelParameterVersion(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDescribeParameters(w http.ResponseWriter, body []byte) {
	var req model.DescribeParametersRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	params, token, err := s.store.DescribeParameters(req.MaxResults, req.NextToken, req.ParameterFilters)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.DescribeParametersResponse{Parameters: params, NextToken: token})
}

func (s *Server) handleDeleteParameter(w http.ResponseWriter, body []byte) {
	var req model.DeleteParameterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	if err := s.store.DeleteParameter(req.Name); err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.DeleteParameterResponse{})
}

func (s *Server) handleDeleteParameters(w http.ResponseWriter, body []byte) {
	var req model.DeleteParametersRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	deleted, invalid := s.store.DeleteParameters(req.Names)
	writeJSON(w, http.StatusOK, model.DeleteParametersResponse{DeletedParameters: deleted, InvalidParameters: invalid})
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

func (s *Server) handleGetParameterHistory(w http.ResponseWriter, body []byte) {
	var req model.GetParameterHistoryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	history, token, err := s.store.GetParameterHistory(req.Name, req.WithDecryption, req.MaxResults, req.NextToken)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.GetParameterHistoryResponse{Parameters: history, NextToken: token})
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

func (s *Server) handleGetParametersByPath(w http.ResponseWriter, body []byte) {
	var req model.GetParametersByPathRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	params, token, err := s.store.GetParametersByPath(req.Path, req.Recursive, req.WithDecryption, req.MaxResults, req.NextToken, req.ParameterFilters)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.GetParametersByPathResponse{Parameters: params, NextToken: token})
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

func (s *Server) handleDescribeSecret(w http.ResponseWriter, body []byte) {
	var req model.DescribeSecretRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.DescribeSecret(req.SecretID)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListSecrets(w http.ResponseWriter, body []byte) {
	var req model.ListSecretsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.ListSecrets(req.MaxResults, req.NextToken, req.Filters)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, body []byte) {
	var req model.DeleteSecretRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.DeleteSecret(req)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRestoreSecret(w http.ResponseWriter, body []byte) {
	var req model.RestoreSecretRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.RestoreSecret(req.SecretID)
	if err != nil {
		writeFromError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUpdateSecretVersionStage(w http.ResponseWriter, body []byte) {
	var req model.UpdateSecretVersionStageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeAWSError(w, http.StatusBadRequest, "ValidationException", "invalid JSON body")
		return
	}

	res, err := s.store.UpdateSecretVersionStage(req)
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

type statusRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	_, _ = s.body.Write(b)
	return s.ResponseWriter.Write(b)
}

func parseErrorType(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	errorType, _ := payload["__type"].(string)
	return errorType
}
