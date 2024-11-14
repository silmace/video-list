# Video List

This is a simple Go server for provide file list, media streaming, and video editing functionalities using FFmpeg.

## Dependencies

- Go 
- FFmpeg (for video processing)
- Node.js and npm (for Vite)

## Build

    
    git https://github.com/silmace/video-list.git
    cd video-edit-server
    npm install
    npm vite build
    go mod tidy
    go build -o video-list
    
## Usage

    
    ./video-list -baseDir /path/to/your/base/directory
baseDir: The base directory to serve files from. Default is '/www'

## Installation
    
    curl -L -o linux-amd64 https://github.com/silmace/video-list/releases/download/beta/linux-amd64
    chmod +x linux-amd64
    mv linux-amd64 /usr/bin/video-list
    apt install ffmpeg -y

    cat <<EOF > /etc/systemd/system/video-list.service
    [Unit]
    Description=video-list
    After=network.target

    [Service]
    Type=simple
    ExecStart=/usr/bin/video-list -baseDir /mnt  # set your baseDir
    PIDFile=/var/run/video-list.pid
    StandardOutput=file:/var/run/myapp-server.log
    StandardError=file:/var/run/myapp-server.log

    [Install]
    WantedBy=multi-user.target
    EOF

    systemctl daemon-reload

    systemctl enable video-list

    systemctl start video-list

## License
This project is licensed under the MIT License.