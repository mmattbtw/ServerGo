FROM golang:1.18 AS build_base

RUN apt-get update && apt install build-essential make wget -y

# Set the Current Working Directory inside the container
WORKDIR /tmp/im

RUN wget https://download.imagemagick.org/ImageMagick/download/ImageMagick.tar.gz && \
	tar -xvf ImageMagick.tar.gz && \
	cd ImageMagick-7.*/ && \
	./configure && \
	make -j$(nproc) && \
	make install && \
	ldconfig /usr/local/lib

WORKDIR /tmp/app

# We want to populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .

RUN go mod download && go install github.com/gobuffalo/packr/v2/packr2@latest

COPY . .

# Build the Go app
RUN packr2 && go build -o seventv

# Start fresh from a smaller image
FROM ubuntu:latest
ENV MAGICK_HOME=/usr
RUN apt-get update && apt-get install -y ca-certificates webp libwebp-dev libpng-dev libjpeg-dev libgif-dev build-essential make wget

WORKDIR /tmp/im

RUN wget https://download.imagemagick.org/ImageMagick/download/ImageMagick.tar.gz && \
        tar -xvf ImageMagick.tar.gz && \
        cd ImageMagick-7.*/ && \
        ./configure && \
        make -j$(nproc) && \
        make install && \
        ldconfig /usr/local/lib


WORKDIR /app

COPY --from=build_base /tmp/app/seventv /app/seventv

# Run the binary program produced by `go install`
CMD ["/app/seventv"]
