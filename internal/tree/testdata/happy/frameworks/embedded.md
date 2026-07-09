# Embedded prompt brief

This document wraps a reusable prompt whose own heading tree must not be read
as document structure.

## The payload

````markdown
# This payload H1 is not ours

Payload prose with its own fenced code at a shorter length.

```go
package main // a nested fence that must not close the outer four-backtick block
```

# A second payload H1
````

## After the payload

An ordinary trailing section, proving the outer fence closed correctly.
