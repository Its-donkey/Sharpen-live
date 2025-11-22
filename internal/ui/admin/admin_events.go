//go:build js && wasm

package admin

import (
	"strings"
	"syscall/js"
)

var adminHandlers []js.Func

func bindAdminEvents() {
	if strings.TrimSpace(adminState.Token) == "" {
		loginForm := getDocument().Call("getElementById", "admin-login-form")
		addAdminHandler(loginForm, "submit", func(this js.Value, args []js.Value) any {
			if len(args) > 0 {
				args[0].Call("preventDefault")
			}
			emailInput := getDocument().Call("getElementById", "admin-email")
			passwordInput := getDocument().Call("getElementById", "admin-password")
			if emailInput.Truthy() {
				adminState.LoginEmail = emailInput.Get("value").String()
			}
			if passwordInput.Truthy() {
				adminState.LoginPassword = passwordInput.Get("value").String()
			}
			go performAdminLogin()
			return nil
		})
		return
	}

	refreshBtn := getDocument().Call("getElementById", "admin-refresh")
	addAdminHandler(refreshBtn, "click", func(js.Value, []js.Value) any {
		go refreshAdminData()
		return nil
	})

	logoutBtn := getDocument().Call("getElementById", "admin-logout")
	addAdminHandler(logoutBtn, "click", func(js.Value, []js.Value) any {
		HandleAdminLogout()
		return nil
	})
	statusCheck := getDocument().Call("getElementById", "admin-status-check")
	addAdminHandler(statusCheck, "click", func(js.Value, []js.Value) any {
		handleRosterStatusCheck()
		return nil
	})

	tabButtons := getDocument().Call("querySelectorAll", "[data-admin-tab]")
	forEachNode(tabButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			tab := this.Get("dataset").Get("adminTab").String()
			handleAdminTabChange(tab)
			return nil
		})
	})

	activityTabs := getDocument().Call("querySelectorAll", "[data-activity-tab]")
	forEachNode(activityTabs, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			tab := this.Get("dataset").Get("activityTab").String()
			handleActivityTabChange(tab)
			return nil
		})
	})

	moderateButtons := getDocument().Call("querySelectorAll", "[data-moderate-action]")
	forEachNode(moderateButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			action := this.Get("dataset").Get("moderateAction").String()
			id := this.Get("dataset").Get("submissionId").String()
			handleModerateAction(action, id)
			return nil
		})
	})

	deleteButtons := getDocument().Call("querySelectorAll", "[data-delete-streamer]")
	forEachNode(deleteButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			id := this.Get("dataset").Get("deleteStreamer").String()
			handleDeleteStreamer(id)
			return nil
		})
	})

	editButtons := getDocument().Call("querySelectorAll", "[data-edit-streamer]")
	forEachNode(editButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			id := this.Get("dataset").Get("editStreamer").String()
			handleToggleStreamerForm(id)
			return nil
		})
	})

	cancelButtons := getDocument().Call("querySelectorAll", "[data-cancel-edit]")
	forEachNode(cancelButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			id := this.Get("dataset").Get("cancelEdit").String()
			handleCancelStreamerEdit(id)
			return nil
		})
	})

	streamerForms := getDocument().Call("querySelectorAll", "[data-streamer-form]")
	forEachNode(streamerForms, func(node js.Value) {
		addAdminHandler(node, "submit", func(this js.Value, args []js.Value) any {
			if len(args) > 0 {
				args[0].Call("preventDefault")
			}
			key := this.Get("dataset").Get("streamerForm").String()
			handleSubmitStreamerForm(key)
			return nil
		})
	})

	streamerFields := getDocument().Call("querySelectorAll", "[data-streamer-field]")
	forEachNode(streamerFields, func(node js.Value) {
		field := node.Get("dataset").Get("streamerField").String()
		streamerID := node.Get("dataset").Get("streamerId").String()
		addAdminHandler(node, "input", func(this js.Value, _ []js.Value) any {
			value := this.Get("value").String()
			handleStreamerFieldChange(streamerID, field, value)
			return nil
		})
		addAdminHandler(node, "change", func(this js.Value, _ []js.Value) any {
			value := this.Get("value").String()
			handleStreamerFieldChange(streamerID, field, value)
			return nil
		})
	})

	platformFields := getDocument().Call("querySelectorAll", "[data-platform-field]")
	forEachNode(platformFields, func(node js.Value) {
		field := node.Get("dataset").Get("platformField").String()
		streamerID := node.Get("dataset").Get("streamerId").String()
		rowID := node.Get("dataset").Get("rowId").String()
		addAdminHandler(node, "input", func(this js.Value, _ []js.Value) any {
			value := this.Get("value").String()
			handlePlatformFieldChange(streamerID, rowID, field, value)
			return nil
		})
	})

	addPlatformButtons := getDocument().Call("querySelectorAll", "[data-add-platform]")
	forEachNode(addPlatformButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			key := this.Get("dataset").Get("addPlatform").String()
			handleAddPlatformRow(key)
			return nil
		})
	})

	removePlatformButtons := getDocument().Call("querySelectorAll", "[data-remove-platform]")
	forEachNode(removePlatformButtons, func(node js.Value) {
		addAdminHandler(node, "click", func(this js.Value, _ []js.Value) any {
			rowID := this.Get("dataset").Get("removePlatform").String()
			owner := this.Get("dataset").Get("platformOwner").String()
			handleRemovePlatformRow(owner, rowID)
			return nil
		})
	})

	settingsForm := getDocument().Call("getElementById", "admin-settings-form")
	addAdminHandler(settingsForm, "submit", func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			args[0].Call("preventDefault")
		}
		handleSettingsSubmit()
		return nil
	})

	settingsFields := getDocument().Call("querySelectorAll", "[data-settings-field]")
	forEachNode(settingsFields, func(node js.Value) {
		field := node.Get("dataset").Get("settingsField").String()
		addAdminHandler(node, "input", func(this js.Value, _ []js.Value) any {
			handleSettingsFieldChange(field, this.Get("value").String())
			return nil
		})
	})

	monitorRefresh := getDocument().Call("getElementById", "admin-monitor-refresh")
	addAdminHandler(monitorRefresh, "click", func(js.Value, []js.Value) any {
		handleMonitorRefresh()
		return nil
	})

	logReconnect := getDocument().Call("getElementById", "admin-logs-reconnect")
	addAdminHandler(logReconnect, "click", func(js.Value, []js.Value) any {
		handleAdminLogsReconnect()
		return nil
	})
	logClear := getDocument().Call("getElementById", "admin-logs-clear")
	addAdminHandler(logClear, "click", func(js.Value, []js.Value) any {
		handleAdminLogsClear()
		return nil
	})

	if adminState.ActivityLogsShouldScroll {
		scrollWebsiteLogFeed()
		adminState.ActivityLogsShouldScroll = false
	}
}

func addAdminHandler(node js.Value, event string, handler func(js.Value, []js.Value) any) {
	if !node.Truthy() {
		return
	}
	fn := js.FuncOf(handler)
	node.Call("addEventListener", event, fn)
	adminHandlers = append(adminHandlers, fn)
}

func releaseAdminHandlers() {
	for _, fn := range adminHandlers {
		fn.Release()
	}
	adminHandlers = nil
}

func forEachNode(list js.Value, fn func(js.Value)) {
	if !list.Truthy() {
		return
	}
	length := list.Get("length").Int()
	for i := 0; i < length; i++ {
		fn(list.Index(i))
	}
}

func scheduleAdminRender() {
	var fn js.Func
	fn = js.FuncOf(func(js.Value, []js.Value) any {
		RenderAdminConsole()
		fn.Release()
		return nil
	})
	js.Global().Call("setTimeout", fn, 0)
}
