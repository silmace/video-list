# Video List

This is a simple Go server for handling video edit requests.

## Dependencies

- Go 
- FFmpeg (for video processing)
- Node.js and npm (for Vite)

## Installation

    ```sh
    git https://github.com/silmace/video-list.git
    cd video-edit-server
    npm install
    npm vite build
    go mod tidy
    go build -o video-list
    ```

## Usage

    ```sh
    ./video-list -baseDir /path/to/your/base/directory
    ```

baseDir: The base directory to serve files from. Default is /www.

## License
This project is licensed under the MIT License.