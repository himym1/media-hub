// One-shot maintainer tool: apply TMDB Identify to completed transfer jobs.
//
//	MEDIA_HUB_DATA_ENCRYPTION_KEY=... go -C backend run ./cmd/emby-identify-completed -db /path/to/media-hub.db
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"media-hub/backend/internal/emby"
	"media-hub/backend/internal/securepayload"
	"media-hub/backend/internal/settings"
)

type jobRow struct {
	Title      string
	Year       int
	MediaType  string
	TMDBID     string
	EmbyItemID string
}

func main() {
	dbPath := flag.String("db", "", "path to media-hub.db")
	userID := flag.Int64("user", 1, "admin user id")
	dryRun := flag.Bool("dry-run", false, "list candidates without calling Emby")
	onlyMissing := flag.Bool("only-missing", true, "skip items that already have the matching Tmdb provider id and a clean title")
	onlyItem := flag.String("item", "", "only identify this Emby item id")
	flag.Parse()
	if strings.TrimSpace(*dbPath) == "" {
		fatalf("missing -db")
	}
	key := strings.TrimSpace(os.Getenv("MEDIA_HUB_DATA_ENCRYPTION_KEY"))
	if key == "" {
		fatalf("MEDIA_HUB_DATA_ENCRYPTION_KEY is required")
	}
	codec, err := securepayload.New(key)
	if err != nil || codec == nil {
		fatalf("invalid encryption key")
	}

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	var sealed string
	err = db.QueryRow(
		`SELECT payload_token FROM provider_credentials WHERE user_id = ? AND provider = ?`,
		*userID, settings.ProviderKey,
	).Scan(&sealed)
	if err != nil {
		fatalf("load settings: %v", err)
	}
	var values settings.Values
	if err := codec.Open(sealed, &values); err != nil {
		fatalf("decrypt settings: %v", err)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(values.Emby.BaseURL), "/")
	apiKey := strings.TrimSpace(values.Emby.APIKey)
	userIDHeader := strings.TrimSpace(values.Emby.UserID)
	if baseURL == "" || apiKey == "" {
		fatalf("emby is not configured in runtime settings")
	}
	if parsed, err := url.Parse(baseURL); err == nil {
		fmt.Printf("emby_host=%s path=%q user_configured=%v\n", parsed.Host, parsed.Path, userIDHeader != "")
	}

	rows, err := db.Query(`
		SELECT title, year, media_type, tmdb_id, emby_item_id
		FROM transfer_jobs
		WHERE state = 'completed'
		  AND archived_at = 0
		  AND emby_item_id != ''
		  AND tmdb_id != ''
		ORDER BY updated_at DESC`)
	if err != nil {
		fatalf("query jobs: %v", err)
	}
	defer rows.Close()

	var jobs []jobRow
	seen := map[string]bool{}
	for rows.Next() {
		var job jobRow
		if err := rows.Scan(&job.Title, &job.Year, &job.MediaType, &job.TMDBID, &job.EmbyItemID); err != nil {
			fatalf("scan job: %v", err)
		}
		if seen[job.EmbyItemID] {
			continue
		}
		if *onlyItem != "" && job.EmbyItemID != *onlyItem {
			continue
		}
		seen[job.EmbyItemID] = true
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		fatalf("iterate jobs: %v", err)
	}
	fmt.Printf("candidates=%d dry_run=%v only_missing=%v\n", len(jobs), *dryRun, *onlyMissing)

	httpClient := &http.Client{Timeout: 90 * time.Second}
	embyClient := emby.NewClient(baseURL, apiKey, 90*time.Second, userIDHeader)
	ctx := context.Background()
	applied, skipped, failed := 0, 0, 0
	for _, job := range jobs {
		provider, name, err := readEmbyItem(ctx, httpClient, baseURL, apiKey, userIDHeader, job.EmbyItemID)
		if err != nil {
			foundID, foundTMDB, foundName, findErr := findEmbyItemByTitle(ctx, httpClient, baseURL, apiKey, userIDHeader, job.Title, job.MediaType, job.Year)
			if findErr != nil {
				fmt.Printf("failed item=%s err=read:%v find:%v\n", job.EmbyItemID, err, findErr)
				failed++
				continue
			}
			fmt.Printf("resolved item=%s -> %s\n", job.EmbyItemID, foundID)
			job.EmbyItemID = foundID
			provider, name = foundTMDB, foundName
		}
		if *onlyMissing && provider == job.TMDBID && !emby.LooksLikeUnidentifiedName(name) {
			fmt.Printf("skip item=%s tmdb=%s\n", job.EmbyItemID, provider)
			skipped++
			continue
		}
		if *dryRun {
			fmt.Printf("would-apply item=%s current_tmdb=%q want=%s unidentified=%v\n", job.EmbyItemID, provider, job.TMDBID, emby.LooksLikeUnidentifiedName(name))
			continue
		}
		if err := embyClient.ApplyTMDBMetadata(ctx, job.EmbyItemID, job.MediaType, job.Title, job.Year, job.TMDBID, true); err != nil {
			fmt.Printf("failed item=%s err=apply:%v\n", job.EmbyItemID, err)
			failed++
			continue
		}
		fmt.Printf("applied item=%s year=%d tmdb=%s\n", job.EmbyItemID, job.Year, job.TMDBID)
		applied++
		time.Sleep(750 * time.Millisecond)
	}
	fmt.Printf("done applied=%d skipped=%d failed=%d\n", applied, skipped, failed)
}

func readEmbyItem(ctx context.Context, client *http.Client, baseURL, apiKey, userID, itemID string) (tmdbID, name string, err error) {
	paths := []string{
		"/Items/" + url.PathEscape(itemID) + "?Fields=" + url.QueryEscape("ProviderIds"),
		"/Items?Ids=" + url.QueryEscape(itemID) + "&Fields=" + url.QueryEscape("ProviderIds"),
	}
	if userID != "" {
		paths = append([]string{
			"/Users/" + url.PathEscape(userID) + "/Items/" + url.PathEscape(itemID) + "?Fields=" + url.QueryEscape("ProviderIds"),
		}, paths...)
	}
	var lastErr error
	for _, path := range paths {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		req.Header.Set("X-Emby-Token", apiKey)
		req.Header.Set("Accept", "application/json")
		resp, doErr := client.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s status %d", path, resp.StatusCode)
			continue
		}
		var single struct {
			ID          string            `json:"Id"`
			Name        string            `json:"Name"`
			ProviderIDs map[string]string `json:"ProviderIds"`
		}
		if err := json.Unmarshal(body, &single); err == nil && single.ID != "" {
			return single.ProviderIDs["Tmdb"], single.Name, nil
		}
		var list struct {
			Items []struct {
				ID          string            `json:"Id"`
				Name        string            `json:"Name"`
				ProviderIDs map[string]string `json:"ProviderIds"`
			} `json:"Items"`
		}
		if err := json.Unmarshal(body, &list); err == nil && len(list.Items) > 0 {
			item := list.Items[0]
			return item.ProviderIDs["Tmdb"], item.Name, nil
		}
		lastErr = fmt.Errorf("%s empty body", path)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("item not found")
	}
	return "", "", lastErr
}

func findEmbyItemByTitle(ctx context.Context, client *http.Client, baseURL, apiKey, userID, title, mediaType string, year int) (itemID, tmdbID, name string, err error) {
	expectedType := "Movie"
	if mediaType == "series" {
		expectedType = "Series"
	}
	query := url.Values{
		"SearchTerm":       {title},
		"IncludeItemTypes": {expectedType},
		"Recursive":        {"true"},
		"Fields":           {"ProviderIds"},
		"Limit":            {"20"},
	}
	if userID != "" {
		query.Set("UserId", userID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/Items?"+query.Encode(), nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("X-Emby-Token", apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("search status %d", resp.StatusCode)
	}
	var list struct {
		Items []struct {
			ID             string            `json:"Id"`
			Name           string            `json:"Name"`
			Type           string            `json:"Type"`
			ProductionYear int               `json:"ProductionYear"`
			ProviderIDs    map[string]string `json:"ProviderIds"`
		} `json:"Items"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return "", "", "", err
	}
	titleFold := strings.ToLower(strings.TrimSpace(title))
	for _, item := range list.Items {
		if item.Type != expectedType || item.ID == "" {
			continue
		}
		if year > 0 && item.ProductionYear > 0 && item.ProductionYear != year {
			continue
		}
		nameFold := strings.ToLower(item.Name)
		if nameFold == titleFold || strings.Contains(nameFold, titleFold) {
			return item.ID, item.ProviderIDs["Tmdb"], item.Name, nil
		}
	}
	return "", "", "", fmt.Errorf("no title match")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
