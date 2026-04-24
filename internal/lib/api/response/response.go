package response

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
)

type Response struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

const (
	StatusOK    = "OK"
	StatusError = "Error"
)

func OK() Response {
	return Response{Status: StatusOK}
}

func Error(msg string) Response {
	return Response{Status: StatusError, Error: msg}
}

func WriteError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	render.Status(r, code)
	render.JSON(w, r, Error(msg))
}

func WriteValidationError(w http.ResponseWriter, r *http.Request, errs validator.ValidationErrors) {
	render.Status(r, http.StatusBadRequest)
	render.JSON(w, r, ValidationError(errs))
}

func ValidationError(errors validator.ValidationErrors) Response {
	var errMessages []string

	for _, err := range errors {
		switch err.ActualTag() {
		case "required":
			errMessages = append(errMessages, fmt.Sprintf("field %s is a required field", err.Field()))
		case "url":
			errMessages = append(errMessages, fmt.Sprintf("field %s must be valid URL", err.Field()))
		default:
			errMessages = append(errMessages, fmt.Sprintf("field %s is not valid", err.Field()))
		}
	}

	return Response{Status: StatusError, Error: strings.Join(errMessages, ", ")}
}
