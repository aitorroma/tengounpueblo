FROM golang:1.26-bookworm AS compilar
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /anuncios . && mkdir -p /datos-vacios

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=compilar /anuncios /anuncios
COPY --from=compilar --chown=65532:65532 /datos-vacios /data
ENV ADDR=:8080 DATA_DIR=/data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/anuncios"]
