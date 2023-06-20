/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package redirect

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"knative.dev/pkg/logging"
)

var orgMappings = map[string]string{
	"appscode": "appscode-images",
	"kubedb":   "kubedb-images",
}

func redact(in http.Header) http.Header {
	h := in.Clone()
	if h.Get("Authorization") != "" {
		h.Set("Authorization", "REDACTED")
	}
	return h
}

func New() http.Handler {
	router := mux.NewRouter()

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			logger := logging.FromContext(ctx)
			logger.Infow("got request",
				"method", req.Method,
				"url", req.URL.String(),
				"header", redact(req.Header))
			next.ServeHTTP(resp, req)
		})
	})

	router.HandleFunc("/v2", v2)
	router.HandleFunc("/v2/", v2)

	router.HandleFunc("/token", token)
	router.HandleFunc("/v2/{org}/{repo}/{rest:.*}", proxy)

	// Redirect any other path to ghcr.io directly.
	// Among other things this will redirect URLs like https://distroless.dev/static:latest
	// to https://ghcr.io/chainguard/static:latest, which will redirect to a useful place.
	// Besides that, any other URL will probably end up serving a 404 from ghcr.io.
	router.HandleFunc("/{rest:.*}", ghpage)
	return router
}

func v2(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	logger := logging.FromContext(ctx)

	out, _ := http.NewRequest(req.Method, "https://ghcr.io/v2/", nil)

	logger.Infow("sending request",
		"method", out.Method,
		"url", out.URL.String(),
		"header", redact(req.Header))
	resp.Header().Set("X-Redirected", out.URL.String())

	back, err := http.DefaultClient.Do(out)
	if err != nil {
		logger.Errorf("Error sending request: %v", err)
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
	defer back.Body.Close()

	logger.Infow("got response",
		"method", out.Method,
		"url", out.URL.String(),
		"status", back.Status,
		"header", redact(back.Header))

	for k, v := range back.Header {
		for _, vv := range v {
			resp.Header().Add(k, vv)
		}
	}

	// Ping responses may include a response header to point to where to get a token, that looks like:
	//   Www-Authenticate: Bearer realm="http://ghcr.io/token",service="ghcr.io"
	//
	// In order for the client to be able to use this, we need to rewrite it to
	// point to our token endpoint, not the upstream:
	//   Www-Authenticate: Bearer realm="http://$HOST/token",service="ghcr.io"
	wwwAuth := back.Header.Get("Www-Authenticate")
	if wwwAuth != "" {
		rewrittenWwwAuth := strings.Replace(wwwAuth, `://ghcr.io/`, fmt.Sprintf(`://%s/`, req.Host), 1)
		resp.Header().Set("Www-Authenticate", rewrittenWwwAuth)
	}

	resp.WriteHeader(back.StatusCode)
	if _, err := io.Copy(resp, back.Body); err != nil {
		logger.Errorf("Error copying response body: %v", err)
	}
}

func token(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	logger := logging.FromContext(ctx)

	vals := req.URL.Query()
	scope := vals.Get("scope")
	for orgKey, ghOrg := range orgMappings {
		if strings.HasPrefix(scope, "repository:"+orgKey+"/") {
			scope = strings.Replace(scope, "repository:"+orgKey+"/", "repository:"+ghOrg+"/", 1)
			break
		}
	}
	vals.Set("scope", scope)

	url := "https://ghcr.io/token?" + vals.Encode()
	out, _ := http.NewRequest(req.Method, url, nil)
	out.Header = req.Header.Clone()

	logger.Infow("sending request",
		"method", out.Method,
		"url", out.URL.String(),
		"header", redact(out.Header))
	resp.Header().Set("X-Redirected", out.URL.String())

	back, err := http.DefaultTransport.RoundTrip(out)
	if err != nil {
		logger.Errorf("Error sending request: %v", err)
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
	defer back.Body.Close()

	logger.Infow("got response",
		"method", out.Method,
		"url", out.URL.String(),
		"status", back.Status,
		"header", redact(back.Header))

	for k, v := range back.Header {
		for _, vv := range v {
			resp.Header().Add(k, vv)
		}
	}

	resp.WriteHeader(back.StatusCode)
	if _, err := io.Copy(resp, back.Body); err != nil {
		logger.Errorf("Error copying response body: %v", err)
	}
}

func proxy(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	logger := logging.FromContext(ctx)

	org := mux.Vars(req)["org"]
	repo := mux.Vars(req)["repo"]
	rest := mux.Vars(req)["rest"]

	url := fmt.Sprintf("https://ghcr.io/v2/%s/%s/%s", orgMappings[org], repo, rest)
	if query := req.URL.Query().Encode(); query != "" {
		url += "?" + query
	}
	out, _ := http.NewRequest(req.Method, url, nil)
	out.Header = req.Header.Clone()

	logger.Infow("sending request",
		"method", out.Method,
		"url", out.URL.String(),
		"header", redact(out.Header))
	resp.Header().Set("X-Redirected", out.URL.String())

	back, err := http.DefaultTransport.RoundTrip(out) // Transport doesn't follow redirects.
	if err != nil {
		logger.Errorf("Error sending request: %v", err)
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}
	defer back.Body.Close()

	logger.Infow("got response",
		"method", req.Method,
		"url", req.URL.String(),
		"status", back.Status,
		"header", redact(back.Header))

	// Copy response headers.
	for k, v := range back.Header {
		for _, vv := range v {
			resp.Header().Add(k, vv)
		}
	}

	// Responses may include a header to point to where to get a token, that looks like:
	//   Www-Authenticate: Bearer realm="http://ghcr.io/token",service="ghcr.io"
	//
	// In order for the client to be able to use this, we need to rewrite it to
	// point to our token endpoint, not the upstream:
	//   Www-Authenticate: Bearer realm="http://$HOST/token",service="ghcr.io"
	wwwAuth := back.Header.Get("Www-Authenticate")
	if wwwAuth != "" {
		rewrittenWwwAuth := strings.Replace(wwwAuth, `://ghcr.io/`, fmt.Sprintf(`://%s/`, req.Host), 1)
		resp.Header().Set("Www-Authenticate", rewrittenWwwAuth)
	}

	// List responses may include a response header to support pagination, that looks like:
	//   Link: </v2/chainguard/static/tags/list?n=100&last=blah>; rel="next">
	//
	// In order for the client to be able to use this link, we need to rewrite it to
	// point to the user's requested repo, not the upstream:
	//   Link: </v2/static/repo/tags/list?n=100&last=blah>; rel="next">
	link := back.Header.Get("Link")
	if link != "" {
		rewrittenLink := link
		for orgKey, ghOrg := range orgMappings {
			if strings.HasPrefix(link, "/v2/"+ghOrg+"/") {
				rewrittenLink = strings.Replace(link, "/v2/"+ghOrg+"/", "/v2/"+orgKey+"/", 1)
				break
			}
		}
		resp.Header().Set("Link", rewrittenLink)
	}

	// If it's a list request, rewrite the response so the name key matches the
	// user's requested repo, otherwise clients will repeatedly request the
	// first page looking for their repo's tags.
	if strings.Contains(req.URL.Path, "/tags/list") {
		var lr listResponse
		if err := json.NewDecoder(back.Body).Decode(&lr); err != nil {
			logger.Errorf("Error decoding list response body: %v", err)
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, ghOrg := range orgMappings {
			if strings.HasPrefix(lr.Name, ghOrg+"/") {
				lr.Name = strings.TrimPrefix(lr.Name, ghOrg+"/")
				break
			}
		}

		// Unset the content-length header from our response, because we're
		// about to rewrite the response to be shorter than the original.
		// This can confuse Cloud Run, which responds with an empty body
		// if the content-length header is wrong in some cases.
		resp.Header().Del("Content-Length")
		resp.WriteHeader(back.StatusCode)
		if err := json.NewEncoder(resp).Encode(lr); err != nil {
			logger.Errorf("Error encoding list response body: %v", err)
			http.Error(resp, err.Error(), http.StatusInternalServerError)
		}

		return
	} else {
		resp.WriteHeader(back.StatusCode)
	}

	// Copy response body.
	if _, err := io.Copy(resp, back.Body); err != nil {
		logger.Errorf("Error copying response body: %v", err)
	}
}

type listResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func ghpage(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	logger := logging.FromContext(ctx)

	url := req.URL.String()
	for orgKey, ghOrg := range orgMappings {
		if req.URL.Path == "/"+orgKey || strings.HasPrefix(req.URL.Path, "/"+orgKey+"/") {
			url = fmt.Sprintf("https://ghcr.io%s", strings.Replace(req.URL.Path, "/"+orgKey, "/"+ghOrg, 1))
			break
		}
	}
	logger.Infof("Redirecting %q to %q", req.URL, url)
	http.Redirect(resp, req, url, http.StatusTemporaryRedirect)
}
