# Video List

This is a simple Go server for handling video edit requests.

## Dependencies

- Go 
- FFmpeg (for video processing)
- Node.js and npm (for Vite)

## Installation

1. Clone the repository:
    ```sh
    git https://github.com/silmace/video-list.git
    cd video-edit-server
    npm vite build
    go mod tidy
    go build -o video-list

## Usage
To start the server, run the following command:
    ```sh
    ./video-list -baseDir /path/to/your/base/directory
baseDir: The base directory to serve files from. Default is /www.

## License
This project is licensed under the MIT License.