# syntax=docker/dockerfile:1
FROM golang:1.19-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./

RUN go build -o ./nightscout_exporter

FROM alpine:latest

LABEL org.opencontainers.image.source=https://github.com/jacobgc/Nightscout-Exporter

ENV TELEMETRY_ADDRESS ":9552"
ENV TELEMETRY_ENDPOINT "/metrics"
ENV NIGHTSCOUT_ENDPOINT ""
ENV NIGHTSCOUT_TOKEN ""
ENV BLOOD_GLUCOSE_STANDARD "UK"

WORKDIR /app

COPY --from=build /app/nightscout_exporter /app/nightscout_exporter

ENTRYPOINT ["./nightscout_exporter"]