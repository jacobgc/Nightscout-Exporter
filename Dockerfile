# syntax=docker/dockerfile:1

FROM golang:1.19-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./

RUN go build -o ./nightscout_exporter

FROM alpine:latest

ENV TELEMETRY_ADDRESS ":9552"
ENV TELEMETRY_ENDPOINT "/metrics"
ENV NIGHTSCOUT_ENDPOINT ""

WORKDIR /app

COPY --from=build /app/nightscout_exporter /app/nightscout_exporter

ENTRYPOINT ["./nightscout_exporter"]