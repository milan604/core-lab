package i18n

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Translator is a thread-safe i18n catalog with interpolation, pluralization, and fallbacks.
type Translator struct {
	mu            sync.RWMutex
	defaultLocale string
	fallbacks     []string
	// store: domain -> locale -> key -> message
	store map[string]map[string]map[string]string
}

// Option customizes Translator on creation.
type Option func(*Translator) error

// New creates a Translator with optional configuration.
func New(opts ...Option) *Translator {
	tr := &Translator{
		defaultLocale: "en",
		store:         make(map[string]map[string]map[string]string),
	}
	for _, opt := range opts {
		_ = opt(tr)
	}
	return tr
}

// WithDefaultLocale sets the default locale (e.g., "en").
func WithDefaultLocale(locale string) Option {
	return func(t *Translator) error {
		if strings.TrimSpace(locale) != "" {
			t.defaultLocale = locale
		}
		return nil
	}
}

// WithFallbackLocales sets fallback locales in preferred order.
func WithFallbackLocales(locales ...string) Option {
	return func(t *Translator) error {
		t.fallbacks = append([]string{}, locales...)
		return nil
	}
}

// WithJSONDir loads messages from a directory with files named <locale>.json into a domain.
func WithJSONDir(domain, dir string) Option {
	return func(t *Translator) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			locale := strings.TrimSuffix(name, ".json")
			if err := t.LoadJSONFile(domain, locale, filepath.Join(dir, name)); err != nil {
				return err
			}
		}
		return nil
	}
}

// LoadJSONFile loads a key->message map from a JSON file into domain/locale.
func (t *Translator) LoadJSONFile(domain, locale, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	t.AddBundle(domain, locale, m)
	return nil
}

// AddBundle merges a bundle of key->message into domain/locale.
func (t *Translator) AddBundle(domain, locale string, bundle map[string]string) {
	if domain == "" {
		domain = "default"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.store[domain]; !ok {
		t.store[domain] = make(map[string]map[string]string)
	}
	if _, ok := t.store[domain][locale]; !ok {
		t.store[domain][locale] = make(map[string]string)
	}
	for k, v := range bundle {
		t.store[domain][locale][k] = v
	}
}

// Add adds a single key/message into domain/locale.
func (t *Translator) Add(domain, locale, key, message string) {
	t.AddBundle(domain, locale, map[string]string{key: message})
}

// Domains returns the registered domains.
func (t *Translator) Domains() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]string, 0, len(t.store))
	for d := range t.store {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Locales returns the locales known for a domain.
func (t *Translator) Locales(domain string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	m := t.store[domain]
	out := make([]string, 0, len(m))
	for loc := range m {
		out = append(out, loc)
	}
	sort.Strings(out)
	return out
}

// splitDomain extracts domain from a key of the form "domain:key".
func splitDomain(key string) (domain, k string) {
	if i := strings.IndexByte(key, ':'); i > 0 {
		return key[:i], key[i+1:]
	}
	return "default", key
}

var placeholderRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_\.]+)\s*\}\}`)

// T translates a key for a locale with optional data and pluralization.
// If data contains a numeric "count" (or n provided), it tries key.one / key.other.
func (t *Translator) T(locale, key string, data map[string]any, n ...int) string {
	if locale == "" {
		locale = t.defaultLocale
	}
	domain, k := splitDomain(key)
	// resolve plural variant
	count := -1
	if len(n) > 0 {
		count = n[0]
	} else if v, ok := data["count"]; ok {
		switch vv := v.(type) {
		case int:
			count = vv
		case int64:
			count = int(vv)
		case float64:
			count = int(vv)
		case string:
			if i, err := strconv.Atoi(vv); err == nil {
				count = i
			}
		}
	}
	keys := []string{k}
	if count >= 0 {
		if count == 1 {
			keys = append([]string{k + ".one"}, keys...)
		} else {
			keys = append([]string{k + ".other"}, keys...)
		}
	}

	// locales search order: requested -> fallbacks -> default
	locales := append([]string{locale}, t.fallbacks...)
	if t.defaultLocale != "" {
		locales = append(locales, t.defaultLocale)
	}

	var msg string
	found := false
	t.mu.RLock()
	for _, loc := range locales {
		bundle := t.store[domain][loc]
		if bundle == nil {
			continue
		}
		for _, kk := range keys {
			if v, ok := bundle[kk]; ok {
				msg = v
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	t.mu.RUnlock()

	if !found {
		// fallback to key itself
		msg = k
	}
	if len(data) == 0 {
		return msg
	}
	return interpolate(msg, data)
}

func interpolate(template string, data map[string]any) string {
	return placeholderRe.ReplaceAllStringFunc(template, func(m string) string {
		sub := placeholderRe.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		key := sub[1]
		// dot path support: a.b -> data[a][b]
		cur, ok := dig(data, key)
		if !ok {
			return m
		}
		return fmt.Sprint(cur)
	})
}

func dig(m map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := mm[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// BestMatch returns the best locale match from an Accept-Language header.
func (t *Translator) BestMatch(acceptLang string) string {
	if strings.TrimSpace(acceptLang) == "" {
		return t.defaultLocale
	}
	// collect available locales
	t.mu.RLock()
	available := map[string]struct{}{}
	for _, locs := range t.store {
		for loc := range locs {
			available[loc] = struct{}{}
		}
	}
	t.mu.RUnlock()

	// parse header: e.g., "en-US,en;q=0.9,fr;q=0.8"
	parts := strings.Split(acceptLang, ",")
	for _, part := range parts {
		lang := strings.TrimSpace(strings.Split(part, ";")[0])
		if lang == "" {
			continue
		}
		// exact
		if _, ok := available[lang]; ok {
			return lang
		}
		// prefix
		base := strings.SplitN(lang, "-", 2)[0]
		for avail := range available {
			if strings.EqualFold(avail, base) || strings.HasPrefix(strings.ToLower(avail), strings.ToLower(base+"-")) {
				return avail
			}
		}
	}
	return t.defaultLocale
}

// Context helpers
type ctxKey string

const localeCtxKey ctxKey = "i18n_locale"

// ContextWithLocale returns a child context with locale stored.
func ContextWithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeCtxKey, locale)
}

// LocaleFromContext returns the stored locale or empty.
func LocaleFromContext(ctx context.Context) string {
	if v := ctx.Value(localeCtxKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GinDetectOptions controls locale detection in middleware.
type GinDetectOptions struct {
	QueryParam string // e.g., "lang"
	HeaderName string // e.g., "Accept-Language"
	CookieName string // e.g., "lang"
}

// defaultGinDetectOptions returns defaults.
func defaultGinDetectOptions() GinDetectOptions {
	return GinDetectOptions{QueryParam: "lang", HeaderName: "Accept-Language", CookieName: "lang"}
}

// GinMiddleware detects locale and stores it into context for handlers.
func (t *Translator) GinMiddleware(opts ...GinDetectOptions) gin.HandlerFunc {
	var o GinDetectOptions
	if len(opts) > 0 {
		o = opts[0]
	} else {
		o = defaultGinDetectOptions()
	}
	return func(c *gin.Context) {
		// order: query -> header -> cookie -> default
		loc := strings.TrimSpace(c.Query(o.QueryParam))
		if loc == "" {
			loc = t.BestMatch(c.GetHeader(o.HeaderName))
		}
		if loc == "" {
			if cookie, err := c.Cookie(o.CookieName); err == nil {
				loc = cookie
			}
		}
		if loc == "" {
			loc = t.defaultLocale
		}
		// attach to gin.Context (std context + gin keys)
		c.Request = c.Request.WithContext(ContextWithLocale(c.Request.Context(), loc))
		c.Set(string(localeCtxKey), loc)
		c.Next()
	}
}

// ErrNotFound can be returned by lookup methods when a key is missing and no fallback desired.
var ErrNotFound = errors.New("i18n: key not found")

// Lookup returns the raw message for a key without interpolation or plural logic.
func (t *Translator) Lookup(locale, key string) (string, error) {
	domain, k := splitDomain(key)
	t.mu.RLock()
	defer t.mu.RUnlock()
	if b := t.store[domain][locale]; b != nil {
		if v, ok := b[k]; ok {
			return v, nil
		}
	}
	return "", ErrNotFound
}
