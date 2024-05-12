# syntax=docker/dockerfile:1

FROM golang:1.22.2 AS build-stage

WORKDIR /app

COPY ../go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=1 go build -o /autobrr-mediahelper


FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /autobrr-mediahelper /autobrr-mediahelper

USER nonroot:nonroot

ENTRYPOINT ["/autobrr-mediahelper"]
