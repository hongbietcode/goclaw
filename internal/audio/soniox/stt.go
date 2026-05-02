// Soniox STT provider — async REST transcription.
// Flow: upload file via Files API → POST /transcriptions → poll until complete → return text.
// Docs: https://soniox.com/docs/stt/async/async-transcription
package soniox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/audio"
)

const (
	sttAPIBase        = "https://api.soniox.com"
	sttDefaultModel   = "stt-async-preview"
	sttDefaultTimeout = 120 * time.Second
	sttPollInterval   = 2 * time.Second
	sttMaxBytes       = 500 << 20 // 500 MB cap
)

// STTProvider transcribes audio via the Soniox async REST API.
type STTProvider struct {
	apiKey    string
	timeoutMs int
}

// NewSTTProvider returns a Soniox STT provider.
func NewSTTProvider(apiKey string, timeoutMs int) *STTProvider {
	if timeoutMs <= 0 {
		timeoutMs = int(sttDefaultTimeout.Milliseconds())
	}
	return &STTProvider{apiKey: apiKey, timeoutMs: timeoutMs}
}

// Name returns the stable provider identifier.
func (p *STTProvider) Name() string { return "soniox" }

// Transcribe uploads the audio file, submits a transcription job, polls until
// complete, and returns the full transcript text.
func (p *STTProvider) Transcribe(ctx context.Context, in audio.STTInput, opts audio.STTOptions) (*audio.TranscriptResult, error) {
	filePath, cleanup, err := resolveSTTFilePath(in)
	if err != nil {
		return nil, fmt.Errorf("soniox stt: resolve input: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err := checkSTTFileSize(filePath, sttMaxBytes); err != nil {
		return nil, fmt.Errorf("soniox stt: %w", err)
	}

	timeout := time.Duration(p.timeoutMs) * time.Millisecond
	hc := &http.Client{Timeout: timeout}

	// Step 1: Upload file via Files API.
	fileID, err := p.uploadFile(ctx, hc, filePath, in)
	if err != nil {
		return nil, fmt.Errorf("soniox stt: upload: %w", err)
	}

	// Step 2: Create transcription job.
	jobID, err := p.createTranscription(ctx, hc, fileID, opts)
	if err != nil {
		return nil, fmt.Errorf("soniox stt: create transcription: %w", err)
	}

	// Step 3: Poll until complete.
	text, lang, dur, err := p.pollTranscription(ctx, hc, jobID)
	if err != nil {
		return nil, fmt.Errorf("soniox stt: poll: %w", err)
	}

	return &audio.TranscriptResult{
		Text:     text,
		Language: lang,
		Duration: dur,
		Provider: "soniox",
	}, nil
}

// uploadFile uploads the audio file and returns the Soniox file_id.
func (p *STTProvider) uploadFile(ctx context.Context, hc *http.Client, filePath string, in audio.STTInput) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	filename := in.Filename
	if filename == "" {
		filename = filepath.Base(filePath)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(fw, f); err != nil {
		return "", fmt.Errorf("write file bytes: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	url := sttAPIBase + "/v1/files"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("empty file_id in response")
	}
	return result.ID, nil
}

// createTranscription submits a transcription job and returns the job ID.
func (p *STTProvider) createTranscription(ctx context.Context, hc *http.Client, fileID string, opts audio.STTOptions) (string, error) {
	model := opts.ModelID
	if model == "" {
		model = sttDefaultModel
	}
	body := map[string]any{
		"file_id": fileID,
		"model":   model,
	}
	if opts.Language != "" {
		body["language"] = opts.Language
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal body: %w", err)
	}

	url := sttAPIBase + "/v1/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("empty job id in response")
	}
	return result.ID, nil
}

// pollTranscription polls GET /v1/transcriptions/{id} until status == "completed",
// then fetches the transcript text from GET /v1/transcriptions/{id}/transcript.
// Returns (text, language, durationSecs, error).
func (p *STTProvider) pollTranscription(ctx context.Context, hc *http.Client, jobID string) (string, string, float64, error) {
	statusURL := sttAPIBase + "/v1/transcriptions/" + jobID

	var durationMs int
	var language string
	for {
		select {
		case <-ctx.Done():
			return "", "", 0, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
		if err != nil {
			return "", "", 0, fmt.Errorf("create poll request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := hc.Do(req)
		if err != nil {
			return "", "", 0, fmt.Errorf("poll request: %w", err)
		}

		var job struct {
			Status      string `json:"status"`
			Language    string `json:"language"`
			DurationMs  int    `json:"audio_duration_ms"`
			ErrorType   string `json:"error_type"`
			ErrorMsg    string `json:"error_message"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&job)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", "", 0, fmt.Errorf("poll API error %d", resp.StatusCode)
		}
		if decodeErr != nil {
			return "", "", 0, fmt.Errorf("parse poll response: %w", decodeErr)
		}

		switch job.Status {
		case "completed":
			durationMs = job.DurationMs
			language = job.Language
		case "error":
			msg := job.ErrorMsg
			if msg == "" {
				msg = job.ErrorType
			}
			if msg == "" {
				msg = "unknown error"
			}
			return "", "", 0, fmt.Errorf("transcription failed: %s", msg)
		default:
			// queued or processing — wait and retry.
			select {
			case <-ctx.Done():
				return "", "", 0, ctx.Err()
			case <-time.After(sttPollInterval):
			}
			continue
		}

		// Fetch the actual transcript text from the transcript endpoint.
		transcriptURL := sttAPIBase + "/v1/transcriptions/" + jobID + "/transcript"
		treq, err := http.NewRequestWithContext(ctx, http.MethodGet, transcriptURL, nil)
		if err != nil {
			return "", "", 0, fmt.Errorf("create transcript request: %w", err)
		}
		treq.Header.Set("Authorization", "Bearer "+p.apiKey)

		tresp, err := hc.Do(treq)
		if err != nil {
			return "", "", 0, fmt.Errorf("transcript request: %w", err)
		}
		var transcript struct {
			Text string `json:"text"`
		}
		decodeErr = json.NewDecoder(tresp.Body).Decode(&transcript)
		tresp.Body.Close()

		if tresp.StatusCode != http.StatusOK {
			return "", "", 0, fmt.Errorf("transcript API error %d", tresp.StatusCode)
		}
		if decodeErr != nil {
			return "", "", 0, fmt.Errorf("parse transcript response: %w", decodeErr)
		}

		durationSecs := float64(durationMs) / 1000.0
		return transcript.Text, language, durationSecs, nil
	}
}

// resolveSTTFilePath returns a usable file path for the input.
func resolveSTTFilePath(in audio.STTInput) (path string, cleanup func(), err error) {
	if in.FilePath != "" {
		return in.FilePath, nil, nil
	}
	if len(in.Bytes) == 0 {
		return "", nil, fmt.Errorf("neither FilePath nor Bytes provided")
	}
	ext := sttExtFromMime(in.MimeType)
	f, err := os.CreateTemp("", "soniox-stt-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}
	if err := os.Chmod(f.Name(), 0600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := f.Write(in.Bytes); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("write temp file: %w", err)
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

// checkSTTFileSize returns an error if the file exceeds maxBytes.
func checkSTTFileSize(path string, maxBytes int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > maxBytes {
		return fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), maxBytes)
	}
	return nil
}

// sttExtFromMime returns a file extension for a MIME type.
func sttExtFromMime(mime string) string {
	switch mime {
	case "audio/ogg", "audio/ogg; codecs=opus":
		return ".ogg"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/wave":
		return ".wav"
	case "audio/mp4", "audio/m4a":
		return ".m4a"
	case "audio/webm":
		return ".webm"
	case "audio/flac":
		return ".flac"
	default:
		return ".bin"
	}
}
