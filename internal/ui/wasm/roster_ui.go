//go:build js && wasm

package wasm

import (
	"fmt"
	"html"
	"strings"
	"syscall/js"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

// TryStreamersWatch attempts to establish a server-sent events subscription using the provided EventSource constructor.
func TryStreamersWatch(ctor js.Value, paths []string) {
	if len(paths) == 0 {
		return
	}
	window := js.Global()
	path := paths[0]
	source := ctor.New(path)
	streamersWatchSource = source
	connected := false

	messageHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		go refreshRoster()
		return nil
	})
	openHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		connected = true
		console := window.Get("console")
		if console.Truthy() {
			console.Call("log", "streamers watch connected", path)
		}
		return nil
	})
	errorHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		console := window.Get("console")
		if console.Truthy() {
			if len(args) > 0 {
				console.Call("warn", "streamers watch error", path, args[0])
			} else {
				console.Call("warn", "streamers watch error", path)
			}
		}
		if !connected && len(paths) > 1 {
			cleanupStreamersWatch()
			go TryStreamersWatch(ctor, paths[1:])
		}
		return nil
	})

	streamersWatchFuncs = append(streamersWatchFuncs, messageHandler, openHandler, errorHandler)
	source.Call("addEventListener", "message", messageHandler)
	source.Call("addEventListener", "open", openHandler)
	source.Call("addEventListener", "error", errorHandler)
}

func renderStreamers(streamers []model.Streamer) {
	if !streamerTable.Truthy() {
		return
	}

	if len(streamers) == 0 {
		setStatusRow("No streamers available at the moment.", false)
		return
	}
	state.StoreRosterSnapshot(streamers)

	var builder strings.Builder
	for _, s := range streamers {
		status := strings.ToLower(strings.TrimSpace(s.Status))
		if status == "" {
			status = "offline"
		}
		label := s.StatusLabel
		if strings.TrimSpace(label) == "" {
			if mapped := model.StatusLabels[status]; mapped != "" {
				label = mapped
			} else if len(status) > 0 {
				label = strings.ToUpper(status[:1]) + status[1:]
			} else {
				label = "Offline"
			}
		}

		builder.WriteString("<tr>")
		builder.WriteString(`<td data-label="Status"><span class="status ` + html.EscapeString(status) + `">`)
		builder.WriteString(html.EscapeString(label))
		builder.WriteString("</span></td>")

		builder.WriteString(`<td data-label="Name"><strong>`)
		builder.WriteString(html.EscapeString(s.Name))
		builder.WriteString("</strong>")
		if strings.TrimSpace(s.Description) != "" {
			builder.WriteString(`<div class="streamer-description">`)
			builder.WriteString(html.EscapeString(s.Description))
			builder.WriteString("</div>")
		}
		builder.WriteString("</td>")

		builder.WriteString(`<td data-label="Streaming Platforms">`)
		if len(s.Platforms) == 0 {
			builder.WriteString("—")
		} else {
			builder.WriteString(`<ul class="platform-list">`)
			for _, p := range s.Platforms {
				name := html.EscapeString(p.Name)
				url := strings.TrimSpace(p.ChannelURL)
				lowerName := strings.ToLower(strings.TrimSpace(p.Name))
				isYouTube := lowerName == "youtube"
				linkClass := "platform-link"
				if isYouTube {
					linkClass += " platform-youtube"
				}
				builder.WriteString("<li>")
				if url != "" {
					builder.WriteString(`<a class="` + linkClass + `" href="` + html.EscapeString(url) + `" target="_blank" rel="noopener noreferrer">`)
				} else {
					builder.WriteString(`<span class="` + linkClass + `" aria-disabled="true">`)
				}
				if isYouTube {
					builder.WriteString(`<span class="platform-icon platform-icon-youtube" aria-hidden="true"></span><span class="platform-label">YouTube</span>`)
				} else {
					builder.WriteString(name)
				}
				if url != "" {
					builder.WriteString(`</a>`)
				} else {
					builder.WriteString(`</span>`)
				}
				builder.WriteString("</li>")
			}
			builder.WriteString("</ul>")
		}
		builder.WriteString("</td>")

		builder.WriteString(`<td data-label="Language"><span class="lang">`)
		if len(s.Languages) == 0 {
			builder.WriteString("—")
		} else {
			builder.WriteString(html.EscapeString(strings.Join(s.Languages, " · ")))
		}
		builder.WriteString("</span></td>")
		builder.WriteString("</tr>")
	}

	streamerTable.Set("innerHTML", builder.String())
}

func setStatusRow(message string, allowRetry bool) {
	if !streamerTable.Truthy() {
		return
	}

	var builder strings.Builder
	builder.WriteString(`<tr><td colspan="4" class="table-status">`)
	builder.WriteString(html.EscapeString(message))
	if allowRetry {
		builder.WriteString(`<br/><button type="button" class="refresh-button" id="retry-fetch">Try again</button>`)
	}
	builder.WriteString("</td></tr>")

	streamerTable.Set("innerHTML", builder.String())

	if allowRetry {
		button := Document.Call("getElementById", "retry-fetch")
		if button.Truthy() {
			button.Call("addEventListener", "click", RefreshFunc)
		}
	}
}

func mainLayout() string {
	currentYear := time.Now().Year()
	return fmt.Sprintf(`
<div class="app-shell">
  <header class="surface site-header">
    <div class="logo-lockup">
      <div class="logo-icon" aria-hidden="true">
        <svg viewBox="0 0 120 120" role="img" aria-labelledby="sharpen-logo-title">
          <title id="sharpen-logo-title">Sharpen Live logo</title>
          <defs>
            <linearGradient id="bladeGradient" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
              <stop offset="0%%" stop-color="#f8fafc" stop-opacity="0.95" />
              <stop offset="55%%" stop-color="#cbd5f5" stop-opacity="0.85" />
              <stop offset="100%%" stop-color="#7dd3fc" stop-opacity="0.95" />
            </linearGradient>
          </defs>
          <path d="M14 68c12-20 38-54 80-58l6 36c-12 6-26 14-41 26l-45-4z" fill="url(#bladeGradient)" stroke="#0f172a" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"></path>
          <path d="M19 76l35 4c-5 5-10 11-15 18l-26-8 6-14z" fill="rgba(15, 23, 42, 0.45)" stroke="#0f172a" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"></path>
          <circle cx="32" cy="92" r="6" fill="#38bdf8"></circle>
          <circle cx="88" cy="36" r="6" fill="#38bdf8"></circle>
        </svg>
      </div>
      <div class="logo-text">
        <h1>Sharpen.Live</h1>
        <p>Streaming Knife Craftsmen</p>
      </div>
    </div>
    <div class="header-actions">
      <a class="cta" href="#streamers">Become a Partner</a>
      <a class="admin-button" href="/admin" aria-label="Open admin console">Admin</a>
    </div>
  </header>

  <main class="surface" id="home-view" aria-labelledby="streamers-title">
    <section class="intro">
      <h2 id="streamers-title">Live Knife Sharpening Studio</h2>
      <p>
        Discover bladesmiths and sharpening artists streaming in real time. Status indicators show who is live, who is prepping off camera, and who is offline.
        Premium partners share their booking links so you can send in your knives for a professional edge.
      </p>
    </section>

    <section class="streamer-table" aria-label="Sharpen Live streamer roster">
      <table>
        <thead>
          <tr>
            <th scope="col">Status</th>
            <th scope="col">Name</th>
            <th scope="col">Streaming Platforms</th>
            <th scope="col">Language</th>
          </tr>
        </thead>
        <tbody id="streamer-rows"></tbody>
      </table>
    </section>

    <div id="submit-streamer-section"></div>
  </main>

  <footer>
    <span>&copy; %d Sharpen Live. All rights reserved.</span>
    <span>Need assistance? <a href="mailto:hello@sharpen.live">hello@sharpen.live</a></span>
  </footer>
</div>
`, currentYear)
}

func adminOnlyLayout() string {
	currentYear := time.Now().Year()
	return fmt.Sprintf(`
<div class="app-shell">
  <header class="surface site-header">
    <div class="logo-lockup">
      <div class="logo-icon" aria-hidden="true">
        <svg viewBox="0 0 120 120" role="img" aria-labelledby="sharpen-logo-admin-title">
          <title id="sharpen-logo-admin-title">Sharpen Live logo</title>
          <defs>
            <linearGradient id="adminBladeGradient" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
              <stop offset="0%%" stop-color="#f8fafc" stop-opacity="0.95" />
              <stop offset="55%%" stop-color="#cbd5f5" stop-opacity="0.85" />
              <stop offset="100%%" stop-color="#7dd3fc" stop-opacity="0.95" />
            </linearGradient>
          </defs>
          <path d="M14 68c12-20 38-54 80-58l6 36c-12 6-26 14-41 26l-45-4z" fill="url(#adminBladeGradient)" stroke="#0f172a" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"></path>
          <path d="M19 76l35 4c-5 5-10 11-15 18l-26-8 6-14z" fill="rgba(15, 23, 42, 0.45)" stroke="#0f172a" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"></path>
          <circle cx="32" cy="92" r="6" fill="#38bdf8"></circle>
          <circle cx="88" cy="36" r="6" fill="#38bdf8"></circle>
        </svg>
      </div>
      <div class="logo-text">
        <h1>Sharpen.Live Admin</h1>
        <p>Operations Console</p>
      </div>
    </div>
  </header>

  <main class="surface" id="admin-view" aria-labelledby="admin-title"></main>

  <footer>
    <span>&copy; %d Sharpen Live. All rights reserved.</span>
    <span>Need assistance? <a href="mailto:hello@sharpen.live">hello@sharpen.live</a></span>
  </footer>
</div>
`, currentYear)
}
