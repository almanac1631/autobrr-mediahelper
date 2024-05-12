# syntax=docker/dockerfile:1

FROM golang:1.22.2 AS build-stage

WORKDIR /app

COPY . .

RUN go mod download

RUN CGO_ENABLED=1 go build -o /autobrr-mediahelper ./cmd/autobrrmediahelper/

FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /autobrr-mediahelper /autobrr-mediahelper

USER nonroot:nonroot

ENTRYPOINT ["/autobrr-mediahelper"]
