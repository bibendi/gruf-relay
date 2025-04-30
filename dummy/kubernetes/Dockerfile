FROM ruby:3.4-slim

ARG APP_PATH=/app

ENV LANG=C.UTF-8 \
    PATH="${APP_PATH}/bin:${PATH}" \
    RAILS_ENV="production" \
    BUNDLE_DEPLOYMENT="1" \
    BUNDLE_PATH="/usr/local/bundle" \
    BUNDLE_WITHOUT="development test"

# Install packages needed to build gems
RUN apt-get update -qq && \
    apt-get install --no-install-recommends -y \
    build-essential \
    git \
    pkg-config \
    libyaml-dev \
    curl \
    libjemalloc2 && \
    rm -rf /var/lib/apt/lists /var/cache/apt/archives

WORKDIR ${APP_PATH}

# Copy gruf-relay gem
COPY ./gem /gem

# Install application gems
COPY ./dummy/Gemfile ./dummy/Gemfile.lock ./
RUN bundle install && \
    rm -rf ~/.bundle/ "${BUNDLE_PATH}"/ruby/*/cache "${BUNDLE_PATH}"/ruby/*/bundler/gems/*/.git

ARG GIT_TAG=0.0.0-dev
ARG BUILD=HEAD
ENV GIT_TAG=$GIT_TAG
ENV BUILD=$BUILD

RUN if [ "$(uname -m)" = "x86_64" ]; then \
    echo "Setting LD_PRELOAD for x86_64 architecture"; \
    echo "LD_PRELOAD=/usr/lib/x86_64-linux-gnu/libjemalloc.so.2" >> /etc/environment; \
  elif [ "$(uname -m)" = "aarch64" ]; then \
    echo "Setting LD_PRELOAD for aarch64 architecture (if jemalloc is needed)"; \
    if [ -f /usr/lib/aarch64-linux-gnu/libjemalloc.so.2 ]; then \
        echo "LD_PRELOAD=/usr/lib/aarch64-linux-gnu/libjemalloc.so.2" >> /etc/environment; \
    else \
        echo "jemalloc not found for aarch64, skipping LD_PRELOAD"; \
    fi; \
  else \
    echo "Unknown architecture: $(uname -m), skipping LD_PRELOAD"; \
  fi

COPY ./dummy ${APP_PATH}

EXPOSE 8080

CMD ["bundle", "exec", "gruf-relay"]
