FROM golang:1.23 AS build

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/web ./cmd/web

FROM gcr.io/distroless/static-debian12

COPY --from=build /go/bin/web /
CMD ["/web"]