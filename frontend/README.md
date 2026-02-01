# Frontend Application

This directory contains the frontend for the C2C Arbitrage Monitor. It is a standalone Single Page Application (SPA) that communicates with the backend API.

## Running the Frontend

Since this is a fully separate frontend, it must be served by a simple web server. The backend (Go application) only provides the API and does **not** serve these files.

### Prerequisites

You need to have one of the following installed:
- Python 3
- Node.js (and `npx`)

### Instructions

1.  **Navigate to the `frontend` directory:**

    ```bash
    cd /Users/junhui/code/go_code/crypto_coin/c2c_monitor/frontend
    ```

2.  **Start a simple web server.** Choose one of the options below.

    **Option 1: Using Python**
    This is the simplest method if you have Python installed.

    ```bash
    # For Python 3
    python3 -m http.server 8080
    ```

    **Option 2: Using Node.js (with `live-server`)**
    `live-server` is a great tool that automatically reloads the page when you make changes to the code.

    ```bash
    # If you don't have live-server installed, this command will download and run it.
    npx live-server --port=8080
    ```

3.  **Open the application in your browser:**

    Once the server is running, open your web browser and go to:

    [http://localhost:8080](http://localhost:8080)

### Configuration

-   The backend API endpoint is configured in `js/config.js`. By default, it points to `http://localhost:8000`.
-   Ensure the Go backend application is running so the frontend can fetch data.
