package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolveNormalizesMovieAndSeriesIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer tmdb-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/search/multi" || request.URL.Query().Get("query") != "范海辛" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":7131,"media_type":"movie","title":"范海辛","original_title":"Van Helsing","release_date":"2004-05-03","poster_path":"/poster.jpg"},{"id":99,"media_type":"person","name":"ignored"}]}`))
	}))
	defer server.Close()

	identities, err := NewClient(server.URL, "tmdb-token", time.Second).Resolve(context.Background(), "范海辛")
	if err != nil {
		t.Fatal(err)
	}
	if len(identities) != 1 || identities[0].TMDBID != "7131" || identities[0].Year != 2004 || identities[0].MediaType != "movie" {
		t.Fatalf("unexpected identities: %#v", identities)
	}
	if identities[0].PosterURL != "https://image.tmdb.org/t/p/w500/poster.jpg" {
		t.Fatalf("unexpected poster URL: %q", identities[0].PosterURL)
	}
}

func TestTrendingAndRecommendationsNormalizeDiscoveryItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer tmdb-token" || request.URL.Query().Get("include_adult") != "false" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/trending/all/week":
			_, _ = w.Write([]byte(`{"results":[{"id":1,"media_type":"movie","title":"Movie","release_date":"2025-01-01","poster_path":"/one.jpg"},{"id":2,"media_type":"person","name":"Ignored"}]}`))
		case "/tv/3/recommendations":
			_, _ = w.Write([]byte(`{"results":[{"id":4,"name":"Series","first_air_date":"2024-02-03"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "tmdb-token", time.Second)
	trending, err := client.Trending(context.Background(), "all", 20)
	if err != nil || len(trending) != 1 || trending[0].MediaType != "movie" || trending[0].Year != 2025 {
		t.Fatalf("trending=%#v err=%v", trending, err)
	}
	recommended, err := client.Recommendations(context.Background(), "series", "3", 20)
	if err != nil || len(recommended) != 1 || recommended[0].TMDBID != "4" || recommended[0].MediaType != "series" {
		t.Fatalf("recommended=%#v err=%v", recommended, err)
	}
}
