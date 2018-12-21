FROM golang:1.11.1-alpine3.8 as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV CGO_ENABLED 0

RUN set -x \
    apk add --no-cache ca-certificates \
		&& apk add --no-cache --virtual \
      .build-deps \
			git \
			gcc \
			libc-dev \
			libgcc

COPY . /app
WORKDIR /app

RUN go build \
		&& mv ghbu /usr/bin/ghbu \
		&& echo "Build complete."

FROM scratch

COPY --from=builder /usr/bin/ghbu /usr/bin/ghbu
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT ["ghbu"]
