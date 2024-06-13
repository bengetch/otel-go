# Tracing propagation with OTel

The `otelhandlers` library just sets up the basic Exporter and Provider resources, which I think
handle stuff like trace ingestion, where the traces get sent, etc. Then in the `otel-go` repository,
each service has a `telemetry.go` file associated with it that calls those setup functions before
separately configuring trace propagation. More specifically, on line 55 of `tracing.go` in the 
`SetupTraces()` function there is this code: 

```go
textPropagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{})
otel.SetTextMapPropagator(textPropagator)
```

That configures some object that passes context info between parent/child spans as they get created. 
Next, we use some automatic instrumentation for the `gin` and `net/http` packages. For `gin`, its 
just this little snippet in the `main()` function:

```go
router.Use(otelgin.Middleware(ServiceName))
```

Then we also need to attach OTel instrumentation to the `http` client that handles requests between
services:

```go
func initHttpClient() {
	/*
		create an http.Client instance with otelhttp transport configured. this transport configuration
		ensures that trace context is correctly propagated across http requests
	*/

	Client = &http.Client{
		Timeout:   time.Second * 10,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}
```

One last thing is to use the `http.NewRequestWithContext()` function (as opposed to the ordinary 
`http.NewRequest()` function) to create HTTP requests, and pass the current context into it. An 
example of this is on line 26 of the `request_handler.go` file that both the `entrypoint_service` 
and `service_a` modules:

```go
req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
```

## Asynchronous trace propagation

There is one endpoint on `entrypoint_service` called `chainedAsyncA` that sends a request to `service_a`,
which immediately responds to `entrypoint_service` before it sends a request to `service_b`. In order for
trace propagation to work for this case, we can't just pass the ordinary `c.Request.Context()` around
between `service_a` and `service_b`, since that context is terminated when `service_a` responds to 
`entrypoint_service`, which is before `service_a` sends a request to `service_b`. 

To ensure that the entire request path shares the same span information, we need to manually extract the span
data from the current context, and use it to create a new context. This is done via the `newContext()` function
in the `main.go` file of `service_a`:

```go
func newContext(oldContext context.Context, header http.Header) context.Context {
	/*
		construct a new context that is not bound to the gin.Request.Context, but contains
		data for current trace
	*/

	propagator := otel.GetTextMapPropagator()
	extractedCtx := propagator.Extract(oldContext, propagation.HeaderCarrier(header))
	newCtx := context.Background()
	carrier := propagation.MapCarrier{}
	propagator.Inject(extractedCtx, carrier)

	return propagator.Extract(newCtx, carrier)
}
```

To see how it is implemented/used, look at the `chainedAsyncRequest()` endpoint function for `service_a`.