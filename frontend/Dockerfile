# AIDEV-NOTE: Multi-stage Dockerfile for StatShed frontend
# Stage 1: Build the application
# Stage 2: Serve with nginx (with runtime BACKEND_URL configuration)

# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# AIDEV-NOTE: Build argument for direct backend connection (WebSocket/API)
# Pass via: docker build --build-arg VITE_BACKEND_URL=http://host:port .
ARG VITE_BACKEND_URL
ENV VITE_BACKEND_URL=$VITE_BACKEND_URL

# Copy package files first for better caching
COPY package.json package-lock.json ./

# Install dependencies
RUN npm ci

# Copy source files
COPY . .

# Build the application (VITE_BACKEND_URL is baked into the bundle)
RUN npm run build

# Production stage
FROM nginx:alpine AS production

# Copy nginx configuration template
COPY nginx.conf.template /etc/nginx/templates/default.conf.template

# Copy built assets from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Default backend URL (can be overridden via environment variable)
ENV BACKEND_URL=http://localhost:7828

# AIDEV-NOTE: Tell nginx's envsubst to ONLY substitute BACKEND_URL
# This prevents nginx variables like $host, $uri from being replaced with empty strings
ENV NGINX_ENVSUBST_FILTER=BACKEND_URL

# Expose port 80
EXPOSE 80

# nginx:alpine image automatically runs envsubst on templates in /etc/nginx/templates/
# and outputs to /etc/nginx/conf.d/ before starting nginx
