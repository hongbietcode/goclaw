// Package soniox implements the Soniox Text-to-Speech REST API.
// Endpoint: POST https://tts-rt.soniox.com/tts
// Docs: https://soniox.com/docs/tts/rest-api/generate-speech
package soniox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/audio"
)

const (
	defaultAPIBase   = "https://tts-rt.soniox.com"
	defaultModel     = "tts-rt-v1"
	defaultVoice     = "Adrian"
	defaultLanguage  = "en"
	defaultFormat    = "mp3"
	defaultTimeoutMs = 30000
)

// Config bundles credentials + TTS defaults for Soniox.
type Config struct {
	APIKey    string
	APIBase   string // default "https://tts-rt.soniox.com"
	Model     string // default "tts-rt-v1"
	Voice     string // default "Adrian"
	Language  string // default "en"
	TimeoutMs int    // default 30000
}

// Provider implements audio.TTSProvider for Soniox.
type Provider struct {
	apiKey    string
	apiBase   string
	model     string
	voice     string
	language  string
	timeoutMs int
}

// NewProvider constructs a Soniox TTS provider with defaults applied.
func NewProvider(cfg Config) *Provider {
	p := &Provider{
		apiKey:    cfg.APIKey,
		apiBase:   cfg.APIBase,
		model:     cfg.Model,
		voice:     cfg.Voice,
		language:  cfg.Language,
		timeoutMs: cfg.TimeoutMs,
	}
	if p.apiBase == "" {
		p.apiBase = defaultAPIBase
	}
	if p.model == "" {
		p.model = defaultModel
	}
	if p.voice == "" {
		p.voice = defaultVoice
	}
	if p.language == "" {
		p.language = defaultLanguage
	}
	if p.timeoutMs <= 0 {
		p.timeoutMs = defaultTimeoutMs
	}
	return p
}

// Name returns the stable provider identifier used by the Manager.
func (p *Provider) Name() string { return "soniox" }

// synthesisRequest is the JSON body for POST /tts.
type synthesisRequest struct {
	Model       string `json:"model"`
	Language    string `json:"language"`
	Voice       string `json:"voice"`
	AudioFormat string `json:"audio_format"`
	Text        string `json:"text"`
	SampleRate  *int   `json:"sample_rate,omitempty"`
	Bitrate     *int   `json:"bitrate,omitempty"`
}

// Synthesize calls POST {apiBase}/tts.
// opts.Params keys: "language" (string), "audio_format" (string), "sample_rate" (int), "bitrate" (int).
// MUST NOT mutate opts.Params — reads only.
func (p *Provider) Synthesize(ctx context.Context, text string, opts audio.TTSOptions) (*audio.SynthResult, error) {
	voice := opts.Voice
	if voice == "" {
		voice = p.voice
	}
	model := opts.Model
	if model == "" {
		model = p.model
	}

	language := resolveString(opts.Params, "language", p.language)
	audioFormat := resolveString(opts.Params, "audio_format", defaultFormat)

	reqBody := synthesisRequest{
		Model:       model,
		Language:    language,
		Voice:       voice,
		AudioFormat: audioFormat,
		Text:        text,
	}

	if sr := resolveInt(opts.Params, "sample_rate", 0); sr > 0 {
		reqBody.SampleRate = &sr
	}
	if br := resolveInt(opts.Params, "bitrate", 0); br > 0 {
		reqBody.Bitrate = &br
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal soniox tts request: %w", err)
	}

	url := p.apiBase + "/tts"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("create soniox tts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	hc := &http.Client{Timeout: time.Duration(p.timeoutMs) * time.Millisecond}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("soniox tts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
		}
		if jsonErr := json.Unmarshal(errBody, &apiErr); jsonErr == nil && apiErr.ErrorMessage != "" {
			return nil, fmt.Errorf("soniox tts error %d [%s]: %s", resp.StatusCode, apiErr.ErrorCode, apiErr.ErrorMessage)
		}
		return nil, fmt.Errorf("soniox tts error %d: %s", resp.StatusCode, string(errBody))
	}

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read soniox tts response: %w", err)
	}

	ext, mime := formatToExtMime(audioFormat)
	return &audio.SynthResult{
		Audio:     audioBytes,
		Extension: ext,
		MimeType:  mime,
	}, nil
}

// formatToExtMime maps Soniox audio_format values to file extension + MIME type.
func formatToExtMime(format string) (string, string) {
	switch format {
	case "mp3":
		return "mp3", "audio/mpeg"
	case "opus":
		return "ogg", "audio/ogg"
	case "aac":
		return "aac", "audio/aac"
	case "flac":
		return "flac", "audio/flac"
	case "wav":
		return "wav", "audio/wav"
	case "pcm_f32le", "pcm_s16le", "pcm_mulaw", "pcm_alaw":
		return "pcm", "audio/pcm"
	default:
		return "mp3", "audio/mpeg"
	}
}

// resolveString reads a string param from params map, falling back to def.
func resolveString(params map[string]any, key, def string) string {
	if params == nil {
		return def
	}
	v, ok := audio.GetNested(params, key)
	if !ok {
		return def
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

// resolveInt reads an int param from params map, falling back to def.
func resolveInt(params map[string]any, key string, def int) int {
	if params == nil {
		return def
	}
	v, ok := audio.GetNested(params, key)
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	}
	return def
}
