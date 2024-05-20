# otel-go
PoC OTel implementation with services written in Go

## configuration

### environment

Inside the `src/` directory, create a `.env` file that matches the `.env.example` file. The contents can be 
identical to those in the `.env.example` file. 

### exporters

Within the `environment` entry for each service defined in the `src/docker-compose.yml` file, there are three 
entries (`TRACES_EXPORTER`, `METRICS_EXPORTER`, `LOGS_EXPORTER`) for configuring how various telemetry types 
get exported. All three entries can take one of the following values: 

1. `otel`: Export telemetry for this type to the `collector` service (which sends all received telemetry to its 
own `stdout` and Datadog by default).
2. `stdout`: Export telemetry for this type to `stdout` of the container that this service is running on.
3. `noop`: Do not export telemetry for this type anywhere.

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

A full list of current endpoints for the `entrypoint_service`:

* `/`: Log a hello message and increment a meter that tracks how many requests have been made to `/`
* `/basicA`: Send a random number to the `/basicRequest` API for `service_a`, which immediately
returns the number back to the `entrypoint_service`.
* `/basicB`: Same as above, but for `service_b`.
* `/chainedA`: Send a random number to the `/chainedRequest` API for `service_a`, which adds another random 
number to it, sends the result to `service_b`, who adds another random number to it before sending the result
back to the `entrypoint_service`.
* `/chainedAsyncA`: Send a random number to the `/chainedAsyncRequest` API for `service_a`, which adds another
random number to it. `service_a` then asynchronously calls the `/chainedRequest` API for `service_b`, and immediately
returns a success message to the `entrypoint_service`. This API demonstrates manual trace propagation, where 
the trace ID for an incoming request must be extracted and used for a new context. The code for this is in 
the `newContext()` function of `service_a`.
* `/inlineTraceEx`: Send a random number to the `/addNumber` API for `service_a`, which adds another random number
to it and returns the result. On the `entrypoint_service`, one of two inline spans are created: one if the returned
number is less than or equal to 5, and another otherwise. This API demonstrates how to manually create traces inside
of application code, as opposed to the automatic instrumentation that is used elsewhere in this repository.

None of them do anything particularly interesting and are only intended to demonstrate various aspects of 
OTel instrumentation.


## TODO
- Write docs on how to configure existing instrumentation (e.g. send telemetry to collector vs service stdout vs noop)