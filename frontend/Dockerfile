# syntax=docker/dockerfile:1.7

FROM --platform=$TARGETPLATFORM nginx:1.27-alpine

WORKDIR /usr/share/nginx/html

# Remove default nginx static files.
RUN rm -rf /usr/share/nginx/html/*

# Copy frontend static assets.
COPY . /usr/share/nginx/html

EXPOSE 80
