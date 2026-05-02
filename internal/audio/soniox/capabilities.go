package soniox

import "github.com/nextlevelbuilder/goclaw/internal/audio"

// sonioxModels lists the available Soniox TTS models.
// See: https://soniox.com/docs/tts/models
var sonioxModels = []string{
	"tts-rt-v1",
}

// sonioxVoices is a curated selection of Soniox voices covering multiple languages.
// See: https://soniox.com/docs/tts/concepts/voices
var sonioxVoices = []audio.VoiceOption{
	{VoiceID: "Adrian", Name: "Adrian (en)"},
	{VoiceID: "Aria", Name: "Aria (en)"},
	{VoiceID: "Atlas", Name: "Atlas (en)"},
	{VoiceID: "Aurora", Name: "Aurora (en)"},
	{VoiceID: "Callum", Name: "Callum (en)"},
	{VoiceID: "Charlie", Name: "Charlie (en)"},
	{VoiceID: "Charlotte", Name: "Charlotte (en)"},
	{VoiceID: "Chris", Name: "Chris (en)"},
	{VoiceID: "Daniel", Name: "Daniel (en)"},
	{VoiceID: "Eric", Name: "Eric (en)"},
	{VoiceID: "Jessica", Name: "Jessica (en)"},
	{VoiceID: "Laura", Name: "Laura (en)"},
	{VoiceID: "Liam", Name: "Liam (en)"},
	{VoiceID: "Lily", Name: "Lily (en)"},
	{VoiceID: "Matilda", Name: "Matilda (en)"},
	{VoiceID: "Nicole", Name: "Nicole (en)"},
	{VoiceID: "River", Name: "River (en)"},
	{VoiceID: "Roger", Name: "Roger (en)"},
	{VoiceID: "Sarah", Name: "Sarah (en)"},
	{VoiceID: "Will", Name: "Will (en)"},
}

// sonioxParams is the param schema for Soniox TTS.
var sonioxParams = []audio.ParamSchema{
	{
		Key:     "language",
		Type:    audio.ParamTypeEnum,
		Label:   "Language",
		Default: "en",
		Enum: []audio.EnumOption{
			{Value: "en", Label: "English"},
			{Value: "es", Label: "Spanish"},
			{Value: "fr", Label: "French"},
			{Value: "de", Label: "German"},
			{Value: "it", Label: "Italian"},
			{Value: "pt", Label: "Portuguese"},
			{Value: "nl", Label: "Dutch"},
			{Value: "pl", Label: "Polish"},
			{Value: "ru", Label: "Russian"},
			{Value: "ja", Label: "Japanese"},
			{Value: "ko", Label: "Korean"},
			{Value: "zh", Label: "Chinese"},
			{Value: "ar", Label: "Arabic"},
			{Value: "hi", Label: "Hindi"},
			{Value: "tr", Label: "Turkish"},
			{Value: "vi", Label: "Vietnamese"},
			{Value: "id", Label: "Indonesian"},
			{Value: "th", Label: "Thai"},
			{Value: "cs", Label: "Czech"},
			{Value: "ro", Label: "Romanian"},
		},
	},
	{
		Key:     "audio_format",
		Type:    audio.ParamTypeEnum,
		Label:   "Output Format",
		Default: "mp3",
		Enum: []audio.EnumOption{
			{Value: "mp3", Label: "MP3"},
			{Value: "opus", Label: "Opus"},
			{Value: "aac", Label: "AAC"},
			{Value: "flac", Label: "FLAC"},
			{Value: "wav", Label: "WAV"},
		},
	},
}

// Capabilities returns the full capability schema for the Soniox TTS provider.
func (p *Provider) Capabilities() audio.ProviderCapabilities {
	return audio.ProviderCapabilities{
		Provider:       "soniox",
		DisplayName:    "Soniox TTS",
		RequiresAPIKey: true,
		Models:         sonioxModels,
		Voices:         sonioxVoices,
		Params:         sonioxParams,
	}
}
