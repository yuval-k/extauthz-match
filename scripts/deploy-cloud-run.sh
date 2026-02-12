#!/usr/bin/env bash
set -euo pipefail

set -x

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROJECT_ID="${PROJECT_ID:-}"
PUBLIC_PROJECT_ID="${PUBLIC_PROJECT_ID:-${PROJECT_ID}}"
REGION="${REGION:-us-central1}"
SERVICE_NAME="${SERVICE_NAME:-extauth-relay}"
VERSION="${VERSION:-0.5}"
IMAGE="${IMAGE:-gcr.io/${PROJECT_ID}/${SERVICE_NAME}:${VERSION}}"
USE_BUILDX="${USE_BUILDX:-true}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
BUILD_SERVER_IMAGE="${BUILD_SERVER_IMAGE:-true}"
SERVER_IMAGE="${SERVER_IMAGE:-gcr.io/${PUBLIC_PROJECT_ID}/extauth-server:${VERSION}}"
if [[ -z "${PROJECT_ID}" ]]; then
  echo "PROJECT_ID is required. Example: PROJECT_ID=my-gcp-project"
  exit 1
fi

if ! command -v gcloud >/dev/null 2>&1; then
  echo "gcloud not found. Install Google Cloud SDK first."
  exit 1
fi

echo "Building relay container image..."
cd "${ROOT_DIR}"

docker buildx create --name extauth-match-builder
trap 'docker buildx rm extauth-match-builder' EXIT

if [[ "${USE_BUILDX}" == "true" ]]; then
  if ! command -v docker >/dev/null 2>&1; then
    echo "docker not found. Install Docker first."
    exit 1
  fi

  if ! docker buildx inspect >/dev/null 2>&1; then
    echo "Docker buildx not available. Install Docker Buildx first."
    exit 1
  fi

  docker buildx build \
    --builder extauth-match-builder \
    --platform "${PLATFORMS}" \
    --file Dockerfile.relay \
    --tag "${IMAGE}" \
    --push \
    .
else
echo
#  gcloud builds submit --tag "${IMAGE}" --file Dockerfile.relay
fi

if [[ "${BUILD_SERVER_IMAGE}" == "true" ]]; then
  echo "Building server container image..."
  if [[ "${USE_BUILDX}" == "true" ]]; then
    docker buildx build \
      --builder extauth-match-builder \
      --platform "${PLATFORMS}" \
      --file Dockerfile \
      --tag "${SERVER_IMAGE}" \
      --push \
      .
  else
    echo
#    gcloud builds submit --tag "${SERVER_IMAGE}" --file Dockerfile
  fi
fi

echo "Deploying to Cloud Run..."
gcloud run deploy "${SERVICE_NAME}" \
  --image "${IMAGE}" \
  --region "${REGION}" \
  --platform managed \
  --allow-unauthenticated \
  --max-instances 1

echo "Done. Fetch the URL with:"
echo "gcloud run services describe ${SERVICE_NAME} --region ${REGION} --format='value(status.url)'"
