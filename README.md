# Nightscout Exporter

A [prometheus](https://prometheus.io/) exporter for [nightscout](https://github.com/nightscout) instances.

## Features
Exports numerical values exported by the `/pebble` nightscout api
* sgv
* trend
* bgdelta

## Environment Variables
* `TELEMETRY_ADDRESS` - binding address (default: `:9552`)
* `TELEMETRY_ENDPOINT` - prometheus metrics endpoint (default: `/metrics`)
* `NIGHTSCOUT_ENDPOINT` - nightscout url (default: `nil`, example: `https://XXXXX-XXXXX-XXXXX.herokuapp.com`)

## TODO
* Add proper logging
* Add proper error handling
* Rewrite to use the proper nightscout api `/api/v1/` rather than the `/pebble` endpoint
* Add authentication to nightscout API calls