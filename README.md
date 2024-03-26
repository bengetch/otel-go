# otel-go
PoC OTel implementation with services written in Go

## usage

Inside the `src` directory, create a `.env` file that matches the `.env.example` file. The contents can be 
identical to those in the `.env.example` file. Then, run `docker compose up --build`.

## endpoints

The `entrypoint` service has several public endpoints that can be accessed via:

```shell
curl http://localhost:5000/<ENDPOINT>
```

A full list of current endpoints:

* `/`
* `/basicA` 
* `/basicB`
* `/chainedA`
* `/chainedAsyncA`

None of them do anything particularly interesting and are only intended to demonstrate various aspects of 
OTel instrumentation.


## TODO

- Write docs on how existing instrumentation works and how to set it up from scratch
- Connect existing telemetry to datadog, or some other visualization backend
- Write docs on how to configure existing instrumentation (e.g. send telemetry to collector vs service stdout vs noop)