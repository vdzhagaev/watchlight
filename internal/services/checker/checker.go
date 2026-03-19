package checker

type CheckRequest struct {
	URL string `json:"url" validate:"required,url"`
}
