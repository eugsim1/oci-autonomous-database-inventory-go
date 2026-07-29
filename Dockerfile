FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/oci-adb-inventory ./cmd/oci-adb-inventory

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/oci-adb-inventory /usr/local/bin/oci-adb-inventory
ENTRYPOINT ["/usr/local/bin/oci-adb-inventory"]
