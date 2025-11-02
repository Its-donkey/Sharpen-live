package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	githubAPIBaseURL = "https://api.github.com"
	defaultOwner     = "Its-donkey"
	defaultRepo      = "Sharpen-live"
	defaultBranch    = "main"
	requestTimeout   = 15 * time.Second
)

type platform struct {
	Name       string `json:"name"`
	ChannelURL string `json:"channelUrl"`
	LiveURL    string `json:"liveUrl"`
}

type submission struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StatusLabel string     `json:"statusLabel"`
	Languages   []string   `json:"languages"`
	Platforms   []platform `json:"platforms"`
}

type submissionResponse struct {
	PullRequestURL string `json:"pullRequestUrl"`
}

type githubErrorResponse struct {
	Message string `json:"message"`
}

type refResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type fileResponse struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type pullRequestResponse struct {
	HTMLURL string `json:"html_url"`
}

var statusDefaultLabels = map[string]string{
	"online":  "Online",
	"busy":    "Workshop",
	"offline": "Offline",
}

var httpClient = &http.Client{
	Timeout: requestTimeout,
}

// Handler is the entry point for the streamer submission API.
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"message": "Method Not Allowed",
		})
		return
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("SHARPEN_GITHUB_TOKEN")
	}
	if token == "" {
		token = os.Getenv("SUBMISSION_GITHUB_TOKEN")
	}

	if token == "" {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"message": "GitHub token is not configured on the server.",
		})
		return
	}

	payload, err := readPayload(r.Body)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}

	owner := firstNonEmpty(os.Getenv("GITHUB_REPO_OWNER"), defaultOwner)
	repo := firstNonEmpty(os.Getenv("GITHUB_REPO_NAME"), defaultRepo)
	baseBranch := firstNonEmpty(os.Getenv("GITHUB_DEFAULT_BRANCH"), defaultBranch)

	ctx := r.Context()
	prURL, err := createStreamerPullRequest(ctx, token, owner, repo, baseBranch, payload)
	if err != nil {
		var githubErr *githubRequestError
		if errors.As(err, &githubErr) && githubErr.StatusCode != 0 {
			respondJSON(w, githubErr.StatusCode, map[string]string{
				"message": githubErr.Error(),
			})
			return
		}

		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"message": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, submissionResponse{
		PullRequestURL: prURL,
	})
}

func readPayload(body io.ReadCloser) (*submission, error) {
	defer body.Close()

	var raw any
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}

	sub, errs := parseSubmission(raw)
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, " "))
	}

	return sub, nil
}

func parseSubmission(raw any) (*submission, []string) {
	data, ok := raw.(map[string]any)
	if !ok {
		return nil, []string{"Payload must be a JSON object."}
	}

	getString := func(key string) string {
		val, exists := data[key]
		if !exists {
			return ""
		}
		switch v := val.(type) {
		case string:
			return strings.TrimSpace(v)
		default:
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}

	name := getString("name")
	description := getString("description")
	status := strings.ToLower(getString("status"))
	statusLabel := getString("statusLabel")

	langs := normalizeLanguages(data["languages"])
	platforms := normalizePlatforms(data["platforms"])

	var errs []string
	if name == "" {
		errs = append(errs, "Streamer name is required.")
	}

	if description == "" {
		errs = append(errs, "Description is required.")
	}

	if status == "" || statusDefaultLabels[status] == "" {
		errs = append(errs, "Status is required and must be one of: online, busy, or offline.")
	}

	if len(langs) == 0 {
		errs = append(errs, "At least one language is required.")
	}

	if len(platforms) == 0 {
		errs = append(errs, "At least one platform with channel and live URLs is required.")
	}

	if statusLabel == "" && status != "" {
		statusLabel = statusDefaultLabels[status]
	}

	return &submission{
		Name:        name,
		Description: description,
		Status:      status,
		StatusLabel: statusLabel,
		Languages:   langs,
		Platforms:   platforms,
	}, errs
}

func normalizeLanguages(value any) []string {
	switch v := value.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, lang := range v {
			str := strings.TrimSpace(fmt.Sprint(lang))
			if str != "" {
				result = append(result, str)
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(v))
		for _, lang := range v {
			str := strings.TrimSpace(lang)
			if str != "" {
				result = append(result, str)
			}
		}
		return result
	case string:
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, lang := range parts {
			str := strings.TrimSpace(lang)
			if str != "" {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

func normalizePlatforms(value any) []platform {
	rawList, ok := value.([]any)
	if !ok {
		return nil
	}

	result := make([]platform, 0, len(rawList))
	for _, entry := range rawList {
		data, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		name := strings.TrimSpace(fmt.Sprint(data["name"]))
		channelURL := strings.TrimSpace(fmt.Sprint(data["channelUrl"]))
		liveURL := strings.TrimSpace(fmt.Sprint(data["liveUrl"]))

		if name == "" || channelURL == "" || liveURL == "" {
			continue
		}

		result = append(result, platform{
			Name:       name,
			ChannelURL: channelURL,
			LiveURL:    liveURL,
		})
	}

	return result
}

func createStreamerPullRequest(
	ctx context.Context,
	token, owner, repo, baseBranch string,
	sub *submission,
) (string, error) {
	baseRef, err := getBaseRef(ctx, token, owner, repo, baseBranch)
	if err != nil {
		return "", err
	}

	branchName, err := ensureBranchExists(ctx, token, owner, repo, baseRef, sub.Name)
	if err != nil {
		return "", err
	}

	fileInfo, err := getStreamersFile(ctx, token, owner, repo, baseBranch)
	if err != nil {
		return "", err
	}

	updatedContent, err := appendStreamer(fileInfo.Content, sub, fileInfo.Encoding)
	if err != nil {
		return "", err
	}

	if err := updateStreamersFile(ctx, token, owner, repo, branchName, updatedContent, fileInfo.SHA, sub.Name); err != nil {
		return "", err
	}

	return openPullRequest(ctx, token, owner, repo, branchName, baseBranch, sub.Name)
}

func getBaseRef(ctx context.Context, token, owner, repo, branch string) (*refResponse, error) {
	var ref refResponse
	endpoint := fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", owner, repo, branch)
	if err := githubRequestJSON(ctx, http.MethodGet, endpoint, token, nil, &ref); err != nil {
		return nil, err
	}

	if ref.Object.SHA == "" {
		return nil, errors.New("unable to locate the base branch SHA")
	}

	return &ref, nil
}

func ensureBranchExists(ctx context.Context, token, owner, repo string, baseRef *refResponse, streamerName string) (string, error) {
	baseSHA := baseRef.Object.SHA
	desired := fmt.Sprintf("feature/items/add-%s", slugify(streamerName))
	branchName := desired

	for attempts := 0; attempts < 5; attempts++ {
		body := map[string]string{
			"ref": fmt.Sprintf("refs/heads/%s", branchName),
			"sha": baseSHA,
		}

		endpoint := fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo)
		err := githubRequestJSON(ctx, http.MethodPost, endpoint, token, body, nil)
		if err == nil {
			return branchName, nil
		}

		var reqErr *githubRequestError
		if errors.As(err, &reqErr) && reqErr.StatusCode == http.StatusUnprocessableEntity {
			branchName = fmt.Sprintf("%s-%s", desired, time.Now().Format("20060102T150405"))
			continue
		}

		return "", err
	}

	return "", errors.New("unable to create a unique branch for the submission")
}

func getStreamersFile(ctx context.Context, token, owner, repo, branch string) (*fileResponse, error) {
	var file fileResponse
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/web/streamers.json?ref=%s", owner, repo, branch)
	if err := githubRequestJSON(ctx, http.MethodGet, endpoint, token, nil, &file); err != nil {
		return nil, err
	}

	return &file, nil
}

func appendStreamer(encodedContent string, sub *submission, encoding string) (string, error) {
	if encoding != "" && !strings.EqualFold(encoding, "base64") {
		return "", fmt.Errorf("unexpected encoding %q for streamers file", encoding)
	}

	content, err := decodeStreamers(encodedContent)
	if err != nil {
		return "", err
	}

	var entries []submission
	if err := json.Unmarshal(content, &entries); err != nil {
		return "", fmt.Errorf("existing streamer list is not valid JSON: %w", err)
	}

	entries = append(entries, submission{
		Name:        sub.Name,
		Description: sub.Description,
		Status:      sub.Status,
		StatusLabel: sub.StatusLabel,
		Languages:   sub.Languages,
		Platforms:   sub.Platforms,
	})

	updated, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("unable to encode streamer list: %w", err)
	}

	return encodeStreamers(updated), nil
}

func updateStreamersFile(ctx context.Context, token, owner, repo, branch, updatedContent, sha, streamerName string) error {
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/web/streamers.json", owner, repo)
	body := map[string]string{
		"message": fmt.Sprintf("feat (items): add streamer %s", streamerName),
		"content": updatedContent,
		"branch":  branch,
		"sha":     sha,
	}

	return githubRequestJSON(ctx, http.MethodPut, endpoint, token, body, nil)
}

func openPullRequest(ctx context.Context, token, owner, repo, branch, baseBranch, streamerName string) (string, error) {
	prBody := strings.Join([]string{
		"## Summary",
		fmt.Sprintf("- add **%s** to the Sharpen Live roster", streamerName),
		"",
		"## Generated By",
		"- Sharpen Live submission form",
	}, "\n")

	payload := map[string]string{
		"title": fmt.Sprintf("feat: add streamer %s", streamerName),
		"head":  branch,
		"base":  baseBranch,
		"body":  prBody,
	}

	var pr pullRequestResponse
	endpoint := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	if err := githubRequestJSON(ctx, http.MethodPost, endpoint, token, payload, &pr); err != nil {
		return "", err
	}

	if pr.HTMLURL == "" {
		return "", errors.New("pull request was created, but no URL was returned")
	}

	return pr.HTMLURL, nil
}

func githubRequestJSON(ctx context.Context, method, endpoint, token string, body any, out any) error {
	reqBody, contentType, err := encodeBody(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, githubAPIBaseURL+endpoint, reqBody)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var ghErr githubErrorResponse
		if err := json.Unmarshal(bodyBytes, &ghErr); err != nil || ghErr.Message == "" {
			return &githubRequestError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("GitHub request failed (%d %s)", resp.StatusCode, resp.Status),
			}
		}

		return &githubRequestError{
			StatusCode: resp.StatusCode,
			Message:    ghErr.Message,
		}
	}

	if out != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, out); err != nil {
			return err
		}
	}

	return nil
}

func encodeBody(body any) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}

	switch b := body.(type) {
	case io.Reader:
		return b, "", nil
	case []byte:
		return bytes.NewReader(b), "application/json", nil
	default:
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return nil, "", err
		}
		return buf, "application/json", nil
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "streamer"
	}

	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, value)

	value = strings.Trim(value, "-")
	if len(value) > 50 {
		value = value[:50]
	}

	if value == "" {
		return "streamer"
	}

	return value
}

func decodeStreamers(encoded string) ([]byte, error) {
	clean := strings.ReplaceAll(encoded, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("unable to decode streamers file: %w", err)
	}
	return decoded, nil
}

func encodeStreamers(content []byte) string {
	return base64.StdEncoding.EncodeToString(content)
}

type githubRequestError struct {
	StatusCode int
	Message    string
}

func (e *githubRequestError) Error() string {
	return e.Message
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
