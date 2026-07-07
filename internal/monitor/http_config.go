package monitor

import (
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
)

type HTTPConfig struct {
	ID          uuid.UUID
	MonitorID   uuid.UUID
	Scheme      HTTPScheme
	Path        Path
	Method      HTTPMethod
	IsEnabled   bool
	Interval    Interval
	Timeout     Timeout
	MaxAttempts MaxAttempts
	Keywords    []string
}

type CreateHTTPConfigInput struct {
	Scheme      string
	Path        string
	Method      string
	IsEnabled   *bool
	Interval    int
	Timeout     int
	MaxAttempts int
	Keywords    []string
}

type UpdateHTTPConfigInput struct {
	Scheme      *string
	Method      *string
	Path        *string
	IsEnabled   *bool
	Interval    *int
	Timeout     *int
	MaxAttempts *int
	Keywords    *[]string
}

func NewHTTPConfig(monitorID uuid.UUID, inConfig CreateHTTPConfigInput) (HTTPConfig, error) {
	interval, err := NewInterval(inConfig.Interval)
	if err != nil {
		return HTTPConfig{}, err
	}
	timeout, err := NewTimeout(inConfig.Timeout)
	if err != nil {
		return HTTPConfig{}, err
	}
	maxAttempts, err := NewMaxAttempts(inConfig.MaxAttempts)
	if err != nil {
		return HTTPConfig{}, err
	}
	scheme, isMatched := parseHTTPScheme(inConfig.Scheme)
	if !isMatched {
		return HTTPConfig{}, ErrInvalidHTTPScheme(inConfig.Scheme)
	}
	method, isMatched := parseHTTPMethod(inConfig.Method)
	if !isMatched {
		return HTTPConfig{}, ErrMethodNotAllowed(inConfig.Method)
	}
	path, err := NewPath(inConfig.Path)
	if err != nil {
		return HTTPConfig{}, err
	}
	if method == MethodHEAD && len(inConfig.Keywords) > 0 {
		return HTTPConfig{}, ErrKeywordsRequireGET
	}
	id, err := uuid.NewV7()
	if err != nil {
		return HTTPConfig{}, err
	}
	return HTTPConfig{
		ID:          id,
		MonitorID:   monitorID,
		Scheme:      scheme,
		Path:        path,
		Method:      method,
		IsEnabled:   enabledOrDefault(inConfig.IsEnabled),
		Interval:    interval,
		Timeout:     timeout,
		MaxAttempts: maxAttempts,
		Keywords:    inConfig.Keywords,
	}, nil
}

// ReconstructHTTPConfig rebuilds an HTTPConfig from trusted, already-validated
// stored values. Scheme/Method are cast directly (their underlying type is
// exported), Path is wrapped without normalization — storage rehydration only.
func ReconstructHTTPConfig(
	id, monitorID uuid.UUID,
	scheme, path, method string,
	isEnabled bool,
	interval, timeout, maxAttempts int,
	keywords []string,
) HTTPConfig {
	return HTTPConfig{
		ID:          id,
		MonitorID:   monitorID,
		Scheme:      HTTPScheme(scheme),
		Path:        ReconstructPath(path),
		Method:      HTTPMethod(method),
		IsEnabled:   isEnabled,
		Interval:    ReconstructInterval(interval),
		Timeout:     ReconstructTimeout(timeout),
		MaxAttempts: ReconstructMaxAttempts(maxAttempts),
		Keywords:    keywords,
	}
}

func buildHTTPConfigs(monitorID uuid.UUID, inConfigs []CreateHTTPConfigInput) ([]HTTPConfig, error) {
	configs := make([]HTTPConfig, 0, len(inConfigs))
	for _, inConfig := range inConfigs {
		c, err := NewHTTPConfig(monitorID, inConfig)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, nil
}

func parseHTTPMethod(s string) (HTTPMethod, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case http.MethodGet:
		return MethodGET, true
	case http.MethodHead:
		return MethodHEAD, true
	default:
		return "", false
	}
}

func parseHTTPScheme(s string) (HTTPScheme, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(SchemeHTTP):
		return SchemeHTTP, true
	case string(SchemeHTTPS):
		return SchemeHTTPS, true
	default:
		return "", false
	}
}

func (cfg HTTPConfig) Update(in UpdateHTTPConfigInput) (HTTPConfig, error) {
	scheme := cfg.Scheme
	if in.Scheme != nil {
		s, ok := parseHTTPScheme(*in.Scheme)
		if !ok {
			return cfg, ErrInvalidHTTPScheme(*in.Scheme)
		}
		scheme = s
	}

	method := cfg.Method
	if in.Method != nil {
		m, ok := parseHTTPMethod(*in.Method)
		if !ok {
			return cfg, ErrMethodNotAllowed(*in.Method)
		}
		method = m
	}

	interval := cfg.Interval
	if in.Interval != nil {
		i, err := NewInterval(*in.Interval)
		if err != nil {
			return cfg, err
		}
		interval = i
	}

	timeout := cfg.Timeout
	if in.Timeout != nil {
		t, err := NewTimeout(*in.Timeout)
		if err != nil {
			return cfg, err
		}
		timeout = t
	}

	maxAttempts := cfg.MaxAttempts
	if in.MaxAttempts != nil {
		ma, err := NewMaxAttempts(*in.MaxAttempts)
		if err != nil {
			return cfg, err
		}
		maxAttempts = ma
	}

	path := cfg.Path
	if in.Path != nil {
		p, err := NewPath(*in.Path)
		if err != nil {
			return cfg, err
		}
		path = p
	}

	enabled := cfg.IsEnabled
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}

	keywords := cfg.Keywords
	if in.Keywords != nil {
		keywords = slices.Clone(*in.Keywords)
	}

	if method == MethodHEAD && len(keywords) > 0 {
		return cfg, ErrKeywordsRequireGET
	}

	cfg.Scheme = scheme
	cfg.Method = method
	cfg.Path = path
	cfg.IsEnabled = enabled
	cfg.Interval = interval
	cfg.Timeout = timeout
	cfg.MaxAttempts = maxAttempts
	cfg.Keywords = keywords
	return cfg, nil
}
