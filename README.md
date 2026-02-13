# ExtAuth Match

A demo project that implements Envoy's external authorization (ext_authz) gRPC interface with a Tinder-like swipeable web UI. Swipe right to approve requests, swipe left to deny them!

## Features

- 🔐 **Envoy ext_authz gRPC server** - Implements the official Envoy authorization API
- 📱 **Mobile-friendly swipe UI** - Approve/deny requests with swipe gestures or buttons
- 🔒 **End-to-end encryption** - AES-256-GCM encryption for secure cloud deployment
- 🌐 **Cloud-ready relay server** - Multi-tenant WebSocket relay for public deployment
- ⚡ **Real-time authorization** - Instant delivery of authorization requests
- 🐳 **Quick setup** - On k8s/kind or docker-compose stack

## Quick Start - Kubernetes Deployment (no need to clone repo)

Install agentgateway (can work with other gateways that support ext_authz too):

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/standard-install.yaml

helm upgrade -i --create-namespace \
  --namespace agentgateway-system \
  --version v2.2.0 agentgateway-crds oci://ghcr.io/kgateway-dev/charts/agentgateway-crds 

helm upgrade -i -n agentgateway-system agentgateway oci://ghcr.io/kgateway-dev/charts/agentgateway \
--version v2.2.0
```

1. **Apply the Kubernetes manifests:**

```bash
# backend html files configmap
kubectl create configmap backend-html \
  --from-file=index.html=<(curl -SsL "https://raw.githubusercontent.com/yuval-k/extauthz-match/refs/heads/master/backend/index.html") \
  --from-file=script.js=<(curl -SsL "https://raw.githubusercontent.com/yuval-k/extauthz-match/refs/heads/master/backend/script.js") \
  --from-file=style.css=<(curl -SsL "https://raw.githubusercontent.com/yuval-k/extauthz-match/refs/heads/master/backend/style.css")

# Apply gateway,http route and policies to setup extauth
kubectl apply -f "https://raw.githubusercontent.com/yuval-k/extauthz-match/refs/heads/master/k8s/allinone.yaml"
```

2. **Browse to the gatewway**

```bash
kubectl port-forward svc/extauth-gateway 8080 &
open http://localhost:8080/ # open this in the browser
```

3. **Scan the QR code** displayed in the page (also appears in the the logs `kubectl logs deployment/extauth-server`):
   - The authz server will display an ASCII QR code with a URL (this is unique URL for each authz server instance)
   - Open this URL on your phone or browser
   - The encryption key is embedded in the URL fragment (after #) and never sent to the server

4. **Swipe right (✓) to approve or left (✗) to deny!**
As the page in the browser sends requests to the gateway, they will appear as cards in the swipe UI. Your swipes will determine whether the requests are allowed or denied in real-time.

## Quick Start - Local Deployment

1. **Start everything:**
   ```bash
   docker compose up
   ```

2. **Open swipe UI** Open the url from the logs in your browser to see the swipe UI. For local dev it should be constant and set to http://localhost:9090/s/630dcd2966c4336691125448#key=AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8= 
you can see it in `docker logs extauth-match-authz-server-1 2>/dev/null`
3. **Make a request to the protected backend:**
   ```bash
   curl http://localhost:10000/
   ```

## Architecture

### Relay-Based with End-to-End Encryption

```
┌─────────────┐                              ┌──────────────┐
│   Browser   │◀──── Encrypted WebSocket ───▶│ Relay Server │
│ (Your Phone)│      (AES-256-GCM)           │  (Port 9090) │
└─────────────┘                              └──────┬───────┘
      ▲                                             │
      │                                             │ Encrypted
      │ Key from URL#fragment                       │ WebSocket
      │ (never sent to server)                      │
      │                                      ┌──────▼───────┐
      └──────────────────────────────────────│ AuthZ Server │
                                             │ (Port 9000)  │
                                             └──────▲───────┘
                                                    │
┌─────────────┐         ┌──────────────────┐       │ ext_authz
│   Client    │────────▶│  AGW Proxy       │───────┘ gRPC
│             │         │  (Port 10000)    │
└─────────────┘         └────────┬─────────┘
                                 │
                         ┌───────▼────────┐
                         │ Backend Server │
                         │  (Port 8081)   │
                         └────────────────┘
```

### Security Model

- **Encryption Key Generation**: A random 256-bit AES key is generated at authz server startup
- **Tenant ID**: Derived from SHA256 hash of the encryption key (first 12 bytes = 24 hex chars)
- **Key Distribution**: Encryption key is embedded in URL fragment (`#key=...`)
  - Fragment is never sent to relay server (browser-only)
  - Enables end-to-end encryption without server-side key management
- **Message Encryption**: All authorization requests/responses encrypted with AES-256-GCM
- **Multi-Tenancy**: Relay server supports multiple concurrent authz servers via tenant IDs

## Services

- **relay-server** (port 9090) - Multi-tenant WebSocket relay for cloud deployment
- **authz-server** (port 9000) - Go service providing ext_authz gRPC API
- **envoy** (ports 10000, 9901) - Envoy proxy with ext_authz filter
- **backend** (port 8081) - Simple nginx serving a protected page

## Endpoints

- `http://localhost:9090/s/{tenantID}` - Relay-hosted swipe UI (key in URL fragment)
- `http://localhost:10000` - Envoy proxy (protected by ext_authz)
- `http://localhost:9901` - Envoy admin interface

## How It Works

1. **Initialization**:
   - Authz server generates encryption key and derives tenant ID
   - Connects to relay server via WebSocket (`/ws/server/{tenantID}`)
   - Displays QR code with URL containing tenant ID and encryption key

2. **Browser Connection**:
   - User scans QR code and opens URL
   - Browser extracts key from URL fragment (client-side only)
   - Connects to relay via WebSocket (`/ws/client/{tenantID}`)

3. **Authorization Flow**:
   - Client makes request → Envoy → ext_authz gRPC call
   - Authz server encrypts request and sends to relay
   - Relay forwards encrypted message to browser
   - Browser decrypts, displays swipe card
   - User swipes → browser encrypts decision → relay → authz server
   - Authz server decrypts and responds to Envoy
   - Envoy allows/denies original request

## Cloud Deployment

The relay server can be deployed to any cloud provider with public access:

1. Deploy relay-server with public IP/domain
2. Update `RELAY_URL` environment variable in docker-compose.yml:
   ```yaml
   environment:
     - RELAY_URL=wss://your-relay-server.com
   ```
3. Run `docker compose up` locally
4. QR code will contain public relay URL for mobile access

### Deploy Relay Server to Cloud Run (recommended)

Cloud Run gives you HTTPS by default on a `*.run.app` URL, and you can add a custom domain later.

1. **Authenticate and select a project:**
   ```bash
   gcloud auth login
   gcloud config set project YOUR_PROJECT_ID
   ```

2. **Deploy the relay container:**
   ```bash
   PROJECT_ID=YOUR_PROJECT_ID bash scripts/deploy-cloud-run.sh
   ```

3. **Get the public URL:**
   ```bash
   gcloud run services describe extauth-relay --region us-central1 --format='value(status.url)'
   ```

4. **Point your authz server to the public relay:**
   - Use `wss://` for the WebSocket URL.
   - Set `BROWSER_BASE_URL` so the QR code points to HTTPS.

   ```bash
   export RELAY_URL=wss://YOUR_SERVICE_URL
   export BROWSER_BASE_URL=https://YOUR_SERVICE_URL
   docker compose up
   ```

Notes:
- The relay listens on the port provided by Cloud Run via `PORT` (set in the deploy script).
- If you rename the service or change regions, adjust the script variables.

## Development

```bash
# Build locally
go build -o bin/authz-server ./cmd/server
go build -o bin/relay-server ./cmd/relay

# Run tests
go test ./...

# Format code
go fmt ./...
```

## Project Structure

```
├── cmd/
│   ├── server/      # AuthZ gRPC server with encryption
│   └── relay/       # Multi-tenant WebSocket relay
├── internal/
│   ├── auth/        # ext_authz gRPC service implementation
│   ├── crypto/      # AES-256-GCM encryption utilities
│   ├── relay/       # Relay client for authz server
│   ├── qrcode/      # ASCII QR code generation
│   └── websocket/   # (legacy) Local WebSocket hub
├── web/
│   └── static/      # HTML/JS swipe UI with Web Crypto API
├── backend/         # Protected nginx backend
├── envoy/           # Envoy configuration
└── docker-compose.yml
```

## Clean Up

Run the following commands:

```bash
kubectl delete configmap backend-html
kubectl delete -f https://raw.githubusercontent.com/yuval-k/extauthz-match/refs/heads/master/k8s/allinone.yaml
```

## License

MIT

---
<div align="center">
    <p>💕 Made with love by the kgateway/agentgateway community 💕 </p>
</div>

