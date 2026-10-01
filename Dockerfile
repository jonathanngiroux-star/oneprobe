# Oneprobe — one static binary, one SQLite file, no runtime deps.
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /oneprobe ./cmd/oneprobe

FROM scratch
COPY --from=build /oneprobe /oneprobe
# The SQLite file lives on a mounted volume.
VOLUME ["/data"]
ENV ONEPROBE_DB=/data/oneprobe.db ONEPROBE_HTTP=0.0.0.0:8080
ENTRYPOINT ["/oneprobe"]
CMD ["serve"]
