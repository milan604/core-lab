# errors

Advanced error wrapping with stack traces.

## Usage
```go
import "github.com/milan604/core-lab/pkg/errors"
err := errors.Wrap(someErr, "failed to process request")
if e, ok := err.(*errors.Error); ok {
  fmt.Println(e.StackTrace())
}
```
