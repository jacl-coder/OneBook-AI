FROM golang:1.25.3 AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG SERVICE
ARG CMD

RUN if [ -z "$SERVICE" ] || [ -z "$CMD" ]; then echo "SERVICE and CMD build args are required" >&2; exit 1; fi
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./services/${SERVICE}/cmd/${CMD}

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /out/app /app

ENV TZ=UTC
ENTRYPOINT ["/app"]
