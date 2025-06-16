# IP Monitor

## Summary

This application monitors your public IP address, network latency, download speed, and upload speed. It periodically checks these metrics and stores them in a SQLite database. A web interface is provided to view the historical data.


## Installation

1.  Make sure you have Go installed.
2.  Clone this repository.

## Building

To build the application, run the following command:

```bash
make build
```

This will create an executable file in the `dist` directory. The filename will be `monitor` (or `monitor.exe` on Windows).

## Running

To run the application, execute the following command:

```bash
./dist/monitor
```

The application will start a web server on port 8080. You can access the web interface at `http://localhost:8080`.


## Contributing

The application consists of three main components:

*   **main.go:** The entry point of the application. It initializes the database and starts the web server and worker.
*   **internal/web/web.go:** Implements the web server that serves the historical data. It uses a SQLite database to fetch and display the records.
*   **internal/worker/worker.go:** Implements the worker that periodically checks the IP address and network metrics and stores them in the Sqlite database.