# otel-go
PoC OTel implementation with services written in Go

## configuration

### environment

Inside the `src/` directory, create a `.env` file that matches the `.env.example` file. The contents can be 
identical to those in the `.env.example` file. If you care about pushing emitted telemetry to Datadog, then
you need to edit the value of `DD_API_KEY` to be a valid Datadog API key. Otherwise, no changes are necessary.

### exporters

Within the `environment` entry for each service defined in the `src/docker-compose.yml` file, there are three 
entries (`TRACES_EXPORTER`, `METRICS_EXPORTER`, `LOGS_EXPORTER`) for configuring how various telemetry types 
get exported. All three entries can take one of the following values: 

1. `otel`: Export telemetry for this type to the `collector` service (which sends all received telemetry to its 
own `stdout` and Datadog by default).
2. `stdout`: Export telemetry for this type to `stdout` of the container that this service is running on.
3. `noop`: Ignore telemetry for this type.

#### example

If the `environment` definition for your `entrypoint_service` looked like this in `docker-compose.yml`:

```shell
      - TRACES_EXPORTER=otel
      - METRICS_EXPORTER=stdout
      - LOGS_EXPORTER=noop
```

Then traces for `entrypoint_service` would be sent to the `collector` instance, metrics would just be logged to `stdout`
of the `entrypoint_service`, and logs would be silenced entirely. 

### run

Run `docker compose up --build` from inside the `src/` directory.

## endpoints

The `entrypoint` service has several public endpoints that can be accessed via:

```shell
curl http://localhost:5000/<ENDPOINT>
```

A full list of current endpoints for the `entrypoint_service` is provided below. None of these endpoints do anything
particularly interesting besides demonstrating various aspects of OTel instrumentation.

* `/`: Log a hello message and increment a meter that tracks how many requests have been made to `/`
* `/basicA`: Send a random number to the `/basicRequest` API for `service_a`, which immediately
returns the number back to the `entrypoint_service`.
* `/basicB`: Same as above, but for `service_b`.
* `/chainedA`: Send a random number to the `/chainedRequest` API for `service_a`, which adds another random 
number to it, sends the result to `service_b`, who adds another random number to it before sending the result
back to the `entrypoint_service`.
* `/chainedAsyncA`: Send a random number to the `/chainedAsyncRequest` API for `service_a`, which adds another
random number to it. `service_a` then asynchronously calls the `/chainedRequest` API for `service_b`, and immediately
returns a success message to the `entrypoint_service`. This API demonstrates **manual** trace propagation, where 
the trace information for an existing request must be extracted and used within a new context that isn't otherwise 
bound to its parent. The code for constructing new traces in this way is in the `newContext()` function of `service_a`.
* `/inlineTraceEx`: Send a random number to the `/addNumber` API for `service_a`, which adds another random number
to it and returns the result. On the `entrypoint_service`, one of two inline spans are created: one if the returned
number is less than or equal to 5, and another otherwise. This API demonstrates how to manually create traces inside
of application code, as opposed to the automatic instrumentation that is used elsewhere in this repository.
