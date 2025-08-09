# i18n — Lightweight Translator with Fallbacks, Interpolation, and Gin Support

A small, fast, and flexible i18n utility:
- JSON bundles per locale and domain (e.g., `default/en.json`, `emails/en.json`)
- Interpolation with `{{placeholders}}` and dot-paths
- Simple pluralization (`key.one`, `key.other`)
- Fallback locales and default locale
- Accept-Language negotiation
- Context helpers and Gin middleware for per-request locale

## Quick Start
```go
tr := i18n.New(
  i18n.WithDefaultLocale("en"),
  i18n.WithFallbackLocales("en"),
  i18n.WithJSONDir("default", "./locales"), // files like locales/en.json, locales/fr.json
)

msg := tr.T("fr", "greeting", map[string]any{"name": "Milan"})
```

`locales/en.json`
```json
{
  "greeting": "Hello, {{name}}!",
  "cart.items.one": "You have {{count}} item",
  "cart.items.other": "You have {{count}} items"
}
```

## Domains
Use `domain:key` to address specific domains, e.g., `emails:welcome.subject`.

```go
tr.Add("emails", "en", "welcome.subject", "Welcome, {{name}}!")
subj := tr.T("en", "emails:welcome.subject", map[string]any{"name":"A"})
```

## Fallbacks
Provide fallback locales; lookup order: requested -> fallbacks -> default -> key.

## Pluralization
Provide `count` in data or pass it as the 4th argument to `T`:
```go
tr.T("en", "cart.items", map[string]any{"count":2}) // uses cart.items.other
tr.T("en", "cart.items", nil, 1) // uses cart.items.one
```

## Accept-Language Negotiation
```go
best := tr.BestMatch("en-US,en;q=0.9,fr;q=0.8")
```

## Gin Middleware
```go
r := gin.New()
r.Use(tr.GinMiddleware()) // stores locale in request context and gin keys
r.GET("/hello", func(c *gin.Context){
  loc := c.GetString("i18n_locale")
  c.JSON(200, gin.H{"message": tr.T(loc, "greeting", map[string]any{"name":"world"})})
})
```

## Loading Bundles
- `WithJSONDir(domain, dir)` loads all `*.json` from dir as `locale.json`
- `LoadJSONFile(domain, locale, path)` to load explicitly
- `AddBundle/Add` to inject programmatically

## API
- `New(opts ...Option) *Translator`
- Options: `WithDefaultLocale`, `WithFallbackLocales`, `WithJSONDir(domain, dir)`
- `(*Translator) T(locale, key, data, n...) string`
- `(*Translator) BestMatch(acceptLang string) string`
- `(*Translator) AddBundle(domain, locale string, bundle map[string]string)`
- `(*Translator) Add(domain, locale, key, message string)`
- `(*Translator) LoadJSONFile(domain, locale, path string) error`
- `(*Translator) GinMiddleware(opts ...GinDetectOptions) gin.HandlerFunc`

## Tips
- Keep messages user-facing; log tech details separately.
- Use domains to separate concerns (e.g., `default`, `emails`, `errors`).
- Consider caching popular translations in memory (already in-memory structure).

---
Private and proprietary. All rights reserved.
