# HTTP request-scoped arena example

This example shows how to use `memoryArena` as request-scoped scratch memory in a normal `net/http` server.

Each incoming request gets its own `MemoryArena[ParsedField]`. The middleware injects that arena into the cloned request context with `memoryArena.InjectRequest`, handlers extract it with `memoryArena.ExtractRequest`, use it for parsing temporary request data, and then the middleware resets the arena at the end of the request.

## Run

From the repository root:

```sh
go run ./examples/http-server
```

## Try it

```sh
curl 'http://localhost:8080/parse?name=kamil&project=memoryArena'
```

Example response:

```json
{
  "path": "/parse",
  "method": "GET",
  "used": 2,
  "remaining": 254,
  "fields": [
    {
      "key": "name",
      "value": "kamil"
    },
    {
      "key": "project",
      "value": "memoryArena"
    }
  ]
}
```

```sh
curl 'http://localhost:8080/sum?n=10&n=20&n=30'
```

Example response:

```json
{
  "fields": [
    {
      "key": "n[0]",
      "value": "10"
    },
    {
      "key": "n[1]",
      "value": "20"
    },
    {
      "key": "n[2]",
      "value": "30"
    }
  ],
  "remaining": 253,
  "sum": 60,
  "used": 3
}
```

## Why this matters

The allocated fields have the same lifetime as the request. Instead of leaving temporary parser objects for the garbage collector, the server clears the whole request-local arena with a single `Reset` after the handler finishes.

This pattern is useful for:

- request parsing;
- small DTO transformation;
- temporary validation state;
- route-local buffers;
- worker-local scratch data.

Do not return pointers or slices allocated from the arena after the request finishes. The middleware resets the arena immediately after `next.ServeHTTP` returns.
