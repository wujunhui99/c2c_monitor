# Frontend Application

This directory contains the frontend for the C2C Arbitrage Monitor. It is a standalone Single Page Application (SPA) that communicates with the backend API.

## Running the Frontend

Since this is a fully separate frontend, it must be served by a simple web server. The backend (Go application) only provides the API and does **not** serve these files.

### Prerequisites

You need to have one of the following installed:
- Python 3
- Node.js (and `npx`)

### Instructions

From the repository root, start the no-cache development server:

```bash
make start-frontend
```

Alternatively, serve the directory directly:

```bash
python3 frontend/dev_server.py 8080 frontend
```

Open:

[http://localhost:8080](http://localhost:8080)

### Configuration

- Copy `js/config.js.example` to the ignored `js/config.js` file when not using `make start-frontend`.
- The default development API endpoint is `http://localhost:8001`.
- Production K3s mounts `deploy/k8s/frontend-config.js`, which uses the same-origin Nginx API proxy.
- Saving configuration, lowering the alert benchmark, and resetting market lows require the backend administrator token.
- The token is stored only in `sessionStorage`, not local storage or source files.
