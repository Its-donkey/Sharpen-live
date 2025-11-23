package model

// Platform describes a distribution channel that carries a streamer.
type Platform struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	ChannelURL string `json:"channelUrl"`
}

// Streamer represents a Sharpen Live roster entry rendered in the public UI.
type Streamer struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StatusLabel string     `json:"statusLabel"`
	Languages   []string   `json:"languages"`
	Platforms   []Platform `json:"platforms"`
}

// WrappedStreamers matches the JSON envelope served by the public roster API.
type WrappedStreamers struct {
	Streamers []Streamer `json:"streamers"`
}

// ServerStreamersResponse models the admin API response containing server-side streamers.
type ServerStreamersResponse struct {
	Streamers []ServerStreamerRecord `json:"streamers"`
}

// ServerStreamerRecord contains the fully-hydrated streamer payload returned by the alert server.
type ServerStreamerRecord struct {
	Streamer  ServerStreamerDetails `json:"streamer"`
	Platforms ServerPlatformDetails `json:"platforms"`
	Status    ServerStatus          `json:"status"`
}

// ServerStreamerDetails captures the core streamer metadata tracked by the alert server.
type ServerStreamerDetails struct {
	ID          string   `json:"id"`
	Alias       string   `json:"alias"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
}

// ServerPlatformDetails lists the platform-specific payloads returned by the alert server.
type ServerPlatformDetails struct {
	YouTube  *ServerYouTubePlatform  `json:"youtube"`
	Facebook *ServerFacebookPlatform `json:"facebook"`
	Twitch   *ServerTwitchPlatform   `json:"twitch"`
}

// ServerYouTubePlatform maps the alert server’s YouTube channel metadata.
type ServerYouTubePlatform struct {
	Handle    string `json:"handle"`
	ChannelID string `json:"channelId"`
	Topic     string `json:"topic"`
}

// ServerFacebookPlatform describes a Facebook page used for streaming.
type ServerFacebookPlatform struct {
	PageID string `json:"pageId"`
}

// ServerTwitchPlatform contains a Twitch username reference.
type ServerTwitchPlatform struct {
	Username string `json:"username"`
}

// ServerStatus conveys the live/offline state reported by the alert server.
type ServerStatus struct {
	Live      bool                 `json:"live"`
	Platforms []string             `json:"platforms"`
	YouTube   *ServerYouTubeStatus `json:"youtube"`
}

// ServerYouTubeStatus provides the active livestream information for YouTube.
type ServerYouTubeStatus struct {
	Live    bool   `json:"live"`
	VideoID string `json:"videoId"`
}

// PlatformFormRow defines a single platform row within the submission/admin forms.
type PlatformFormRow struct {
	ID         string
	Name       string
	Preset     string
	ChannelURL string
	Handle     string
	ChannelID  string
	HubSecret  string
}

// PlatformFieldError tracks platform-specific validation errors on form rows.
type PlatformFieldError struct {
	Channel bool
}

// SubmitFormErrors groups validation flags for the public submission form.
type SubmitFormErrors struct {
	Name        bool
	Description bool
	Languages   bool
	Platforms   map[string]PlatformFieldError
}

// SubmitFormState represents the fully-rendered submission form state.
type SubmitFormState struct {
	Open          bool
	Name          string
	Description   string
	Languages     []string
	Platforms     []PlatformFormRow
	Errors        SubmitFormErrors
	ResultMessage string
	ResultState   string
	Submitting    bool
}

// CreateStreamerRequest is the payload posted when creating a streamer through the API.
type CreateStreamerRequest struct {
	Streamer  StreamerPayload   `json:"streamer"`
	Platforms StreamerPlatforms `json:"platforms"`
}

// StreamerPayload contains the editable streamer fields used by the API.
type StreamerPayload struct {
	Alias       string   `json:"alias"`
	Description string   `json:"description,omitempty"`
	Languages   []string `json:"languages,omitempty"`
}

// StreamerPlatforms lists the URLs associated with a new streamer.
type StreamerPlatforms struct {
	URL string `json:"url,omitempty"`
}

// CreateStreamerResponse captures the alert server’s response to a POST /api/streamers request.
type CreateStreamerResponse struct {
	Streamer struct {
		ID    string `json:"id"`
		Alias string `json:"alias"`
	} `json:"streamer"`
}

// MetadataRequest is a helper payload used when requesting metadata scraping.
type MetadataRequest struct {
	URL string `json:"url"`
}

// MetadataResponse is the server-provided metadata for a streaming channel.
type MetadataResponse struct {
	Description string `json:"description"`
	Title       string `json:"title"`
	Handle      string `json:"handle"`
	ChannelID   string `json:"channelId"`
}

// AdminSubmission represents a pending roster submission awaiting moderation.
type AdminSubmission struct {
	ID          string   `json:"id"`
	Alias       string   `json:"alias"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	PlatformURL string   `json:"platformUrl"`
	SubmittedAt string   `json:"submittedAt"`
}

// AdminSubmissionPayload mirrors the editable fields used when updating a streamer from the admin console.
type AdminSubmissionPayload struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StatusLabel string     `json:"statusLabel"`
	Languages   []string   `json:"languages"`
	Platforms   []Platform `json:"platforms"`
}

// AdminSettings captures the persisted alert server configuration that is editable through the console.
type AdminSettings struct {
	ListenAddr              string `json:"listenAddr"`
	AdminToken              string `json:"adminToken"`
	AdminEmail              string `json:"adminEmail"`
	AdminPassword           string `json:"adminPassword"`
	YouTubeAPIKey           string `json:"youtubeApiKey"`
	YouTubeAlertsCallback   string `json:"youtubeAlertsCallback"`
	YouTubeAlertsSecret     string `json:"youtubeAlertsSecret"`
	YouTubeAlertsVerifyPref string `json:"youtubeAlertsVerifyPrefix"`
	YouTubeAlertsVerifySuff string `json:"youtubeAlertsVerifySuffix"`
	YouTubeAlertsHubURL     string `json:"youtubeAlertsHubUrl"`
	DataDir                 string `json:"dataDir"`
	StaticDir               string `json:"staticDir"`
	StreamersFile           string `json:"streamersFile"`
	SubmissionsFile         string `json:"submissionsFile"`
}

// AdminSettingsUpdate represents a diff of settings fields to update.
type AdminSettingsUpdate map[string]string

// AdminMonitorEvent records a single alert pulled from the alert server monitor feed.
type AdminMonitorEvent struct {
	ID        int    `json:"id"`
	Platform  string `json:"platform"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// YouTubeLeaseStatus describes the subscription lease health for a YouTube channel.
type YouTubeLeaseStatus struct {
	Alias        string `json:"alias"`
	Handle       string `json:"handle"`
	ChannelID    string `json:"channelId"`
	LeaseStart   string `json:"leaseStart"`
	LeaseExpires string `json:"leaseExpires"`
	Status       string `json:"status"`
	Expired      bool   `json:"expired"`
	ExpiringSoon bool   `json:"expiringSoon"`
	StartDate    string `json:"startDate,omitempty"`
}

// AdminActivityLog stores the display text for stdout/stderr entries shown in the console.
type AdminActivityLog struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

// LoginResponse holds the admin JWT returned by the alert server.
type LoginResponse struct {
	Token string `json:"token"`
}

// LanguageOption provides label/value pairs for the language picker.
type LanguageOption struct {
	Label string
	Value string
}

// AdminStatus reflects a toaster-style notice shown inside the admin console.
type AdminStatus struct {
	Message string
	Tone    string
}

// AdminStatusCheckResult summarises the outcome of a live-status refresh.
type AdminStatusCheckResult struct {
	Checked int `json:"checked"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
	Updated int `json:"updated"`
	Failed  int `json:"failed"`
}

// AdminStreamerForm represents the editable streamer fields in the admin console.
type AdminStreamerForm struct {
	ID               string
	Name             string
	Description      string
	Status           string
	StatusLabel      string
	StatusLabelDirty bool
	LanguagesInput   string
	Platforms        []PlatformFormRow
	Visible          bool
	Saving           bool
	Error            string
}

// AdminViewState tracks the global admin console state shared across the WASM app.
type AdminViewState struct {
	Token                    string
	LoginEmail               string
	LoginPassword            string
	Status                   AdminStatus
	Loading                  bool
	StatusCheckRunning       bool
	ActiveTab                string
	ActivityTab              string
	ActivityLogs             []AdminActivityLog
	ActivityLogsError        string
	ActivityLogsShouldScroll bool
	Submissions              []AdminSubmission
	Streamers                []Streamer
	YouTubeLeases            map[string]YouTubeLeaseStatus
	StreamerForms            map[string]*AdminStreamerForm
	Settings                 *AdminSettings
	SettingsDraft            *AdminSettings
	SettingsLoading          bool
	SettingsSaving           bool
	MonitorEvents            []AdminMonitorEvent
	MonitorLoading           bool
}

const (
	// MaxLanguages limits the number of languages a streamer can include.
	MaxLanguages = 8
	// MaxPlatforms limits the number of platforms per streamer form.
	MaxPlatforms = 8
)

// StatusLabels maps backend status values to human-friendly strings.
var StatusLabels = map[string]string{
	"online":  "Online",
	"busy":    "Workshop",
	"offline": "Offline",
}

// LanguageOptions enumerates all selectable languages presented in the UI.
var LanguageOptions = []LanguageOption{
	{Label: "English", Value: "English"},
	{Label: "Afrikaans", Value: "Afrikaans"},
	{Label: "Albanian / Shqip", Value: "Albanian"},
	{Label: "Amharic / አማርኛ (Amariññā)", Value: "Amharic"},
	{Label: "Armenian / Հայերեն (Hayeren)", Value: "Armenian"},
	{Label: "Azerbaijani / Azərbaycanca", Value: "Azerbaijani"},
	{Label: "Basque / Euskara", Value: "Basque"},
	{Label: "Belarusian / Беларуская (Belaruskaya)", Value: "Belarusian"},
	{Label: "Bosnian / Bosanski", Value: "Bosnian"},
	{Label: "Bulgarian / Български (Bŭlgarski)", Value: "Bulgarian"},
	{Label: "Catalan / Català", Value: "Catalan"},
	{Label: "Cebuano / Binisaya", Value: "Cebuano"},
	{Label: "Croatian / Hrvatski", Value: "Croatian"},
	{Label: "Czech / Čeština", Value: "Czech"},
	{Label: "Danish / Dansk", Value: "Danish"},
	{Label: "Dutch / Nederlands", Value: "Dutch"},
	{Label: "Estonian / Eesti", Value: "Estonian"},
	{Label: "Filipino / Tagalog", Value: "Filipino"},
	{Label: "Finnish / Suomi", Value: "Finnish"},
	{Label: "Galician / Galego", Value: "Galician"},
	{Label: "Georgian / ქართული (Kartuli)", Value: "Georgian"},
	{Label: "German / Deutsch", Value: "German"},
	{Label: "Greek / Ελληνικά (Elliniká)", Value: "Greek"},
	{Label: "Gujarati / ગુજરાતી (Gujarātī)", Value: "Gujarati"},
	{Label: "Haitian Creole / Kreyòl Ayisyen", Value: "Haitian Creole"},
	{Label: "Hebrew / עברית (Ivrit)", Value: "Hebrew"},
	{Label: "Hmong / Hmoob", Value: "Hmong"},
	{Label: "Hungarian / Magyar", Value: "Hungarian"},
	{Label: "Icelandic / Íslenska", Value: "Icelandic"},
	{Label: "Igbo", Value: "Igbo"},
	{Label: "Italian / Italiano", Value: "Italian"},
	{Label: "Japanese / 日本語 (Nihongo)", Value: "Japanese"},
	{Label: "Javanese / Basa Jawa", Value: "Javanese"},
	{Label: "Kannada / ಕನ್ನಡ (Kannaḍa)", Value: "Kannada"},
	{Label: "Kazakh / Қазақ (Qazaq)", Value: "Kazakh"},
	{Label: "Khmer / ខ្មែរ (Khmer)", Value: "Khmer"},
	{Label: "Kinyarwanda", Value: "Kinyarwanda"},
	{Label: "Korean / 한국어 (Hangugeo)", Value: "Korean"},
	{Label: "Kurdish / کوردی", Value: "Kurdish"},
	{Label: "Lao / ລາວ", Value: "Lao"},
	{Label: "Latvian / Latviešu", Value: "Latvian"},
	{Label: "Lithuanian / Lietuvių", Value: "Lithuanian"},
	{Label: "Luxembourgish / Lëtzebuergesch", Value: "Luxembourgish"},
	{Label: "Macedonian / Македонски (Makedonski)", Value: "Macedonian"},
	{Label: "Malay / Bahasa Melayu", Value: "Malay"},
	{Label: "Malayalam / മലയാളം (Malayāḷam)", Value: "Malayalam"},
	{Label: "Maltese / Malti", Value: "Maltese"},
	{Label: "Marathi / मराठी (Marāṭhī)", Value: "Marathi"},
	{Label: "Mongolian / Монгол (Mongol)", Value: "Mongolian"},
	{Label: "Nepali / नेपाली (Nepālī)", Value: "Nepali"},
	{Label: "Norwegian / Norsk", Value: "Norwegian"},
	{Label: "Pashto / پښتو", Value: "Pashto"},
	{Label: "Persian / فارسی (Fārsi)", Value: "Persian"},
	{Label: "Polish / Polski", Value: "Polish"},
	{Label: "Punjabi / ਪੰਜਾਬੀ (Pañjābī)", Value: "Punjabi"},
	{Label: "Romanian / Română", Value: "Romanian"},
	{Label: "Serbian / Српски (Srpski)", Value: "Serbian"},
	{Label: "Sinhala / සිංහල (Siṁhala)", Value: "Sinhala"},
	{Label: "Slovak / Slovenčina", Value: "Slovak"},
	{Label: "Slovenian / Slovenščina", Value: "Slovenian"},
	{Label: "Somali / Soomaali", Value: "Somali"},
	{Label: "Swahili / Kiswahili", Value: "Swahili"},
	{Label: "Swedish / Svenska", Value: "Swedish"},
	{Label: "Tamil / தமிழ் (Tamiḻ)", Value: "Tamil"},
	{Label: "Telugu / తెలుగు (Telugu)", Value: "Telugu"},
	{Label: "Thai / ไทย", Value: "Thai"},
	{Label: "Turkish / Türkçe", Value: "Turkish"},
	{Label: "Ukrainian / Українська (Ukrayins'ka)", Value: "Ukrainian"},
	{Label: "Urdu / اردو", Value: "Urdu"},
	{Label: "Uzbek / Oʻzbek", Value: "Uzbek"},
	{Label: "Vietnamese / Tiếng Việt", Value: "Vietnamese"},
	{Label: "Welsh / Cymraeg", Value: "Welsh"},
	{Label: "Xhosa / isiXhosa", Value: "Xhosa"},
	{Label: "Yoruba / Yorùbá", Value: "Yoruba"},
	{Label: "Zulu / isiZulu", Value: "Zulu"},
}
