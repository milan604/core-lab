# app

Builder-style service bootstrap for `core-lab` services.

It standardizes the common startup flow:

`logger -> config -> runtime config -> observability -> audit -> validator -> engine -> server`

## What It Solves
- Removes repeated `main.go` startup boilerplate from service repos
- Keeps config, observability, audit, and server wiring consistent
- Lets services inject only their own setup, routes, middleware, and shutdown logic

## Usage
```go
app.New(serviceName, serviceVersion).
  WithPort(servicePort).
  WithConfigFile("env/config.json").
  WithConfigOptions(config.WithDotEnv("")).
  WithRuntimeConfig(runtimeconfig.ResolveOptions{
    BootstrapPath: "env/config.json",
    Required: runtimeconfig.RequiredFromEnv(),
  }).
  OnSetup(func(ctx app.Context) (*app.SetupResult, error) {
    db, err := openDB(ctx.Config)
    if err != nil {
      return nil, err
    }

    svc := service.New(ctx.Logger, db)

    return &app.SetupResult{
      Middleware: []gin.HandlerFunc{
        middleware.SetService(svc),
      },
      Shutdown: []app.ShutdownFunc{
        func(app.Context) error {
          return db.Close()
        },
      },
    }, nil
  }).
  WithRoutes(func(engine *gin.Engine, ctx app.Context) {
    router.AddRoutes(ctx.Logger, ctx.Config, engine)
  }).
  OnShutdown(func(ctx app.Context) error {
    return nil
  }).
  Run()
```

## Lifecycle
- `OnSetup` runs after shared infrastructure is initialized
- `SetupResult.Middleware` is applied before routes
- `OnPostSetup` runs after routes are registered but before server start
- `OnShutdown` and `SetupResult.Shutdown` run after the server stops, in reverse order

## Notes
- Add `WithConfigOptions(config.WithDotEnv(""))` only for services that already rely on dotenv loading
- `SetupResult.Shutdown` is the best place to close resources created during setup
- `OnShutdown` is useful for broader service-level cleanup that depends on app context

---
Private and proprietary. All rights reserved.
