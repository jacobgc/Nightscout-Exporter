# Nightscout Exporter

A [prometheus](https://prometheus.io/) exporter for [nightscout](https://github.com/nightscout) instances.

## Features
Exports numerical values exported by the `/api/v1/entries` nightscout api
* sgv (mg/dL or mmol/L)
* trend
* bgdelta

## Environment Variables
* `TELEMETRY_ADDRESS` - binding address (default: `:9552`)
* `TELEMETRY_ENDPOINT` - prometheus metrics endpoint (default: `/metrics`)
* `NIGHTSCOUT_ENDPOINT` - nightscout url (default: `nil`, example: `https://XXXXX-XXXXX-XXXXX.herokuapp.com`)
* `NIGHTSCOUT_TOKEN` - the token to use when scraping data (default: '', example: `exporter-4de3d6cceab4db62`')
* `BLOOD_GLUCOSE_STANDARD` - the standard to use for blood glucose values (UK: `mmol/L` US: `mg/dL`) (default: `UK` Accepts: `UK`/`US`)
## TODO
* Nothing