package forms

import (
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"sync"
)

func AvailableLanguageOptions(selected []string) []model.LanguageOption {
	out := make([]model.LanguageOption, 0, len(model.LanguageOptions))
	set := map[string]struct{}{}
	for _, v := range selected {
		set[v] = struct{}{}
	}
	for _, opt := range model.LanguageOptions {
		if _, ok := set[opt.Value]; !ok {
			out = append(out, opt)
		}
	}
	return out
}

func ContainsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

var (
	langOnce sync.Once
	langMap  map[string]string
)

func DisplayLanguage(value string) string {
	langOnce.Do(func() {
		langMap = map[string]string{}
		for _, o := range model.LanguageOptions {
			langMap[o.Value] = o.Label
		}
	})
	if lbl := langMap[value]; lbl != "" {
		return lbl
	}
	return value
}
