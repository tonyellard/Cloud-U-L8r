// SPDX-License-Identifier: Apache-2.0

// Package awserrors provides standardized AWS-compatible error response
// formatting for Cloud-U-L8r service emulators.
package awserrors

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// XMLError represents an S3-style XML error response.
type XMLError struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestId string   `xml:"RequestId,omitempty"`
}

// XMLErrorResponse represents an SQS-style XML error response (wrapped).
type XMLErrorResponse struct {
	XMLName xml.Name       `xml:"ErrorResponse"`
	Error   XMLErrorDetail `xml:"Error"`
}

// XMLErrorDetail is the inner error element in an SQS-style ErrorResponse.
type XMLErrorDetail struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// JSONError represents an AWS JSON protocol error response.
// Serializes to {"__type": "ErrorCode", "message": "..."}.
type JSONError struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// WriteXML writes an S3-style XML error response.
// Used by essthree.
func WriteXML(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
	resp := XMLError{
		Code:      code,
		Message:   message,
		Resource:  r.URL.Path,
		RequestId: r.Header.Get("X-Request-ID"),
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(statusCode)
	xml.NewEncoder(w).Encode(resp)
}

// WriteXMLWrapped writes an SQS-style XML ErrorResponse.
// Used by ess-queue-ess.
func WriteXMLWrapped(w http.ResponseWriter, code, message string, statusCode int) {
	resp := XMLErrorResponse{}
	resp.Error.Type = "Sender"
	resp.Error.Code = code
	resp.Error.Message = message

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(statusCode)

	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	encoder.Encode(resp)
}

// WriteCloudFrontXML writes a CloudFront-style XML error response.
// Used by cloudfauxnt.
func WriteCloudFrontXML(w http.ResponseWriter, code, message, requestID string, statusCode int) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("X-Amz-Cf-Id", requestID)
	w.Header().Set("Server", "CloudFauxnt")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(statusCode)

	errorXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <RequestId>%s</RequestId>
</Error>`, code, message, requestID)

	io.WriteString(w, errorXML)
}

// WriteJSON writes an AWS JSON protocol error response.
// Used by kay-vee and admin-console.
func WriteJSON(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(JSONError{Type: code, Message: message})
}

// WriteJSONGeneric writes a generic JSON error (non-AWS format).
// Used by admin-console for its own API errors.
func WriteJSONGeneric(w http.ResponseWriter, statusCode int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// MapGoErrorToAWS translates a Go error string with "ErrorType: message" format
// into the appropriate AWS error type and HTTP status code, then writes the response.
// Used by kay-vee for its error-type-to-status mapping.
func MapGoErrorToAWS(w http.ResponseWriter, err error) {
	if err == nil {
		WriteJSON(w, http.StatusInternalServerError, "InternalFailure", "unknown error")
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
	case "ValidationException", "ParameterAlreadyExists", "InvalidParameterException",
		"InvalidRequestException", "ResourceExistsException":
		status = http.StatusBadRequest
	case "ParameterNotFound", "ResourceNotFoundException":
		status = http.StatusNotFound
	}

	WriteJSON(w, status, typeName, msg)
}
