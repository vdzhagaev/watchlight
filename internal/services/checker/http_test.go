package checker

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type handleConfig struct {
	code    int
	body    string
	timeout time.Duration
}

type want struct {
	method        string
	code          int
	err           error
	reachable     bool
	foundKeywords []string
}

func TestHttpChecker(t *testing.T) {
	tests := []struct {
		name          string
		handlerConfig handleConfig
		want          want
		reqTimeout    time.Duration
		reqKeywords   []string
	}{
		{
			name: "ok",
			handlerConfig: handleConfig{
				code: http.StatusOK,
			},
			want: want{
				method:    http.MethodHead,
				code:      http.StatusOK,
				reachable: true,
			},
		},
		{
			name: "service unavailable",
			handlerConfig: handleConfig{
				code: http.StatusServiceUnavailable,
			},
			want: want{
				code:      http.StatusServiceUnavailable,
				method:    http.MethodHead,
				reachable: true,
			},
		},
		{
			name:        "keyword found",
			reqKeywords: []string{"golang", "google"},
			handlerConfig: handleConfig{
				code: http.StatusOK,
				body: `<div class="Footer-linkColumn">
            <a href="/wiki/#the-go-community" class="Footer-link Footer-link--primary" aria-describedby="footer-description">
              Connect
            </a>
              <a href="https://bsky.app/profile/golang.org" class="Footer-link" aria-describedby="footer-description">
                Bluesky
              </a>
              <a href="https://hachyderm.io/@golang" class="Footer-link" aria-describedby="footer-description">
                Mastodon
              </a>
              <a href="https://www.twitter.com/golang" class="Footer-link" aria-describedby="footer-description">
                Twitter
              </a>
              <a href="https://github.com/golang" class="Footer-link" aria-describedby="footer-description">
                GitHub
              </a>
              <a href="https://invite.slack.golangbridge.org/" class="Footer-link" aria-describedby="footer-description">
                Slack
              </a>
              <a href="https://reddit.com/r/golang" class="Footer-link" aria-describedby="footer-description">
                r/golang
              </a>
              <a href="https://www.meetup.com/pro/go" class="Footer-link" aria-describedby="footer-description">
                Meetup
              </a>
              <a href="https://golangweekly.com/" class="Footer-link" aria-describedby="footer-description">
                Golang Weekly
              </a>
          </div>`,
			},
			want: want{
				method:        http.MethodGet,
				code:          http.StatusOK,
				reachable:     true,
				foundKeywords: []string{"golang"},
			},
		},
		{
			name:        "keyword not found",
			reqKeywords: []string{"google"},
			handlerConfig: handleConfig{
				code: http.StatusOK,
				body: `<div class="Footer-linkColumn">
            <a href="/wiki/#the-go-community" class="Footer-link Footer-link--primary" aria-describedby="footer-description">
              Connect
            </a>
              <a href="https://bsky.app/profile/golang.org" class="Footer-link" aria-describedby="footer-description">
                Bluesky
              </a>
              <a href="https://hachyderm.io/@golang" class="Footer-link" aria-describedby="footer-description">
                Mastodon
              </a>
              <a href="https://www.twitter.com/golang" class="Footer-link" aria-describedby="footer-description">
                Twitter
              </a>
              <a href="https://github.com/golang" class="Footer-link" aria-describedby="footer-description">
                GitHub
              </a>
              <a href="https://invite.slack.golangbridge.org/" class="Footer-link" aria-describedby="footer-description">
                Slack
              </a>
              <a href="https://reddit.com/r/golang" class="Footer-link" aria-describedby="footer-description">
                r/golang
              </a>
              <a href="https://www.meetup.com/pro/go" class="Footer-link" aria-describedby="footer-description">
                Meetup
              </a>
              <a href="https://golangweekly.com/" class="Footer-link" aria-describedby="footer-description">
                Golang Weekly
              </a>
          </div>`,
			},
			want: want{
				method:        http.MethodGet,
				code:          http.StatusOK,
				reachable:     true,
				foundKeywords: []string{},
			},
		},
		{
			name:       "timeout - head",
			reqTimeout: 20 * time.Millisecond,
			handlerConfig: handleConfig{
				timeout: 100 * time.Millisecond,
			},
			want: want{
				method:    http.MethodHead,
				err:       ErrTimeout,
				reachable: false,
			},
		},
		{
			name:        "timeout - get",
			reqTimeout:  20 * time.Millisecond,
			reqKeywords: []string{"golang"},
			handlerConfig: handleConfig{
				timeout: 100 * time.Millisecond,
			},
			want: want{
				method:    http.MethodGet,
				err:       ErrTimeout,
				reachable: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			methodCh := make(chan string, 1)
			srv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					methodCh <- r.Method
					w.Header().Set("Content-Type", "text/html")
					select {
					case <-time.After(tc.handlerConfig.timeout):
						w.WriteHeader(tc.handlerConfig.code)
						io.WriteString(w, tc.handlerConfig.body)
					case <-r.Context().Done():
						return
					}
				}),
			)
			defer srv.Close()

			res, _ := HTTPChecker{}.Check(t.Context(), CheckRequest{
				URL:      srv.URL,
				Timeout:  tc.reqTimeout,
				Keywords: tc.reqKeywords,
			})

			var gotMethod string
			select {
			case gotMethod = <-methodCh:
			case <-time.After(time.Second):
			}

			if gotMethod != tc.want.method {
				t.Errorf("Method = %s, want %s", gotMethod, tc.want.method)
			}
			if res.StatusCode != tc.want.code {
				t.Errorf("StatusCode = %d, want %d", res.StatusCode, tc.want.code)
			}
			if !errors.Is(res.Err, tc.want.err) {
				t.Errorf("Error = %v, want %v", res.Err, tc.want.err)
			}
			if res.Reachable != tc.want.reachable {
				t.Errorf("Reachable = %t, want %t", res.Reachable, tc.want.reachable)
			}
			if !reflect.DeepEqual(res.FoundKeywords, tc.want.foundKeywords) {
				t.Errorf("FoundKeywords = %v, want %v", res.FoundKeywords, tc.want.foundKeywords)
			}
		})
	}
}
