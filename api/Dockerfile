FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

FROM alpine:3.20
COPY --from=build /out/api /usr/local/bin/api
COPY static /static
WORKDIR /
ENV HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["api"]
