package tilenol

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFilterLayersByName(t *testing.T) {
	layers := []Layer{
		Layer{Name: "a"},
		Layer{Name: "b"},
		Layer{Name: "c"},
	}
	filtered := filterLayersByNames(layers, []string{"a", "c", "doesntexist"})
	if len(filtered) != 2 {
		t.Errorf("Filtered layers should have been length 2, instead was: %d", len(filtered))
	}
	if filtered[0].Name != "a" {
		t.Errorf("Expected first layer to be 'a', got: %s", filtered[0].Name)
	}
	if filtered[1].Name != "c" {
		t.Errorf("Expected second layer to be 'c', got: %s", filtered[1].Name)
	}
}

func TestFilterLayersByZoom(t *testing.T) {
	layers := []Layer{
		Layer{Name: "a", Minzoom: 10},
		Layer{Name: "b", Maxzoom: 10},
		Layer{Name: "c", Minzoom: 5, Maxzoom: 15},
	}

	filteredZ0 := filterLayersByZoom(layers, 0)
	if len(filteredZ0) != 1 {
		t.Error("z = 0")
	}
	filteredZ5 := filterLayersByZoom(layers, 5)
	if len(filteredZ5) != 2 {
		t.Error("z = 5")
	}
	filteredZ10 := filterLayersByZoom(layers, 10)
	if len(filteredZ10) != 3 {
		t.Error("z = 10")
	}
	filteredZ20 := filterLayersByZoom(layers, 20)
	if len(filteredZ20) != 1 {
		t.Error("z = 20")
	}
}

func TestCalculateSimplificationThreshold(t *testing.T) {
	if calculateSimplificationThreshold(0, 20, 0) > MaxSimplify {
		t.Error("Simplification exceeds MaxSimplify")
	}
	if calculateSimplificationThreshold(0, 20, 20) < MinSimplify {
		t.Error("Simplification is below MinSimplify")
	}
}

func TestCachedHandler(t *testing.T) {
	server := &Server{Cache: NewInMemoryCache()}
	var requests []interface{}
	handler := func(context.Context, io.Writer, *http.Request) error {
		requests = append(requests, nil)
		return nil
	}
	cachedHandler := server.cached(handler)
	for i := 0; i < 100; i++ {
		body := ioutil.NopCloser(bytes.NewReader([]byte{}))
		r := httptest.NewRequest("GET", "/_all/0/0/0.mvt", body)
		w := httptest.NewRecorder()
		cachedHandler.ServeHTTP(w, r)
		res := w.Result()
		if res.StatusCode != 200 {
			t.Error("Unsuccessful status code")
		}
	}
	if len(requests) != 1 {
		t.Error("Request not cached")
	}
}

func TestUnCachedHandler(t *testing.T) {
	server := &Server{Cache: &NilCache{}}
	var requests []interface{}
	handler := func(context.Context, io.Writer, *http.Request) error {
		requests = append(requests, nil)
		return nil
	}
	cachedHandler := server.cached(handler)
	for i := 0; i < 100; i++ {
		body := ioutil.NopCloser(bytes.NewReader([]byte{}))
		r := httptest.NewRequest("GET", "/_all/0/0/0.mvt", body)
		w := httptest.NewRecorder()
		cachedHandler.ServeHTTP(w, r)
		res := w.Result()
		if res.StatusCode != 200 {
			t.Error("Unsuccessful status code")
		}
	}
	if len(requests) != 100 {
		t.Error("Requests should not be cached")
	}
}

func TestAPI(t *testing.T) {
	server := &Server{Cache: &NilCache{}}
	api, internal := server.setupRoutes()

	// Test tile endpoint
	body := ioutil.NopCloser(bytes.NewReader([]byte{}))
	r := httptest.NewRequest("GET", "/_all/0/0/0.mvt", body)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	res := w.Result()
	if res.StatusCode != 200 {
		t.Error("Non-200 tile response")
	}

	// Test healthcheck
	body = ioutil.NopCloser(bytes.NewReader([]byte{}))
	r = httptest.NewRequest("GET", "/healthcheck", body)
	w = httptest.NewRecorder()
	internal.ServeHTTP(w, r)
	res = w.Result()
	if res.StatusCode != 200 {
		t.Error("Non-200 healthcheck response")
	}
}
