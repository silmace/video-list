import express from 'express';
import cors from 'cors';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import fs from 'fs/promises';
import { createReadStream, existsSync, statSync } from 'fs';
import ffmpeg from 'fluent-ffmpeg';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const app = express();
const PORT = 3000;

// Middleware
app.use(cors());
app.use(express.json());

// Base directory for video files
const BASE_DIR = join(__dirname, '../videos');

// Ensure base directory exists
try {
  await fs.access(BASE_DIR);
} catch {
  await fs.mkdir(BASE_DIR, { recursive: true });
}

// List files in directory
app.get('/api/files', async (req, res) => {
  try {
    const requestedPath = req.query.path || '/';
    const fullPath = join(BASE_DIR, requestedPath);
    
    // Prevent directory traversal
    if (!fullPath.startsWith(BASE_DIR)) {
      return res.status(403).json({ error: 'Access denied' });
    }

    const files = await fs.readdir(fullPath);
    const fileList = await Promise.all(
      files.map(async (file) => {
        const filePath = join(fullPath, file);
        const stats = await fs.stat(filePath);
        const relativePath = filePath.substring(BASE_DIR.length);

        return {
          name: file,
          path: relativePath,
          isDirectory: stats.isDirectory(),
          size: stats.size,
          modifiedTime: stats.mtime
        };
      })
    );

    res.json(fileList);
  } catch (error) {
    console.error('Error reading directory:', error);
    res.status(500).json({ error: 'Failed to read directory' });
  }
});

// Stream video file
app.get('/api/video', (req, res) => {
  try {
    const videoPath = join(BASE_DIR, req.query.path);
    
    // Prevent directory traversal
    if (!videoPath.startsWith(BASE_DIR)) {
      return res.status(403).json({ error: 'Access denied' });
    }

    if (!existsSync(videoPath)) {
      return res.status(404).json({ error: 'Video not found' });
    }

    const stat = statSync(videoPath);
    const fileSize = stat.size;
    const range = req.headers.range;

    if (range) {
      const parts = range.replace(/bytes=/, '').split('-');
      const start = parseInt(parts[0], 10);
      const end = parts[1] ? parseInt(parts[1], 10) : fileSize - 1;
      const chunksize = end - start + 1;
      const stream = createReadStream(videoPath, { start, end });

      res.writeHead(206, {
        'Content-Range': `bytes ${start}-${end}/${fileSize}`,
        'Accept-Ranges': 'bytes',
        'Content-Length': chunksize,
        'Content-Type': 'video/mp4'
      });

      stream.pipe(res);
    } else {
      res.writeHead(200, {
        'Content-Length': fileSize,
        'Content-Type': 'video/mp4'
      });
      createReadStream(videoPath).pipe(res);
    }
  } catch (error) {
    console.error('Error streaming video:', error);
    res.status(500).json({ error: 'Failed to stream video' });
  }
});

// Process video segments
app.post('/api/edit-video', async (req, res) => {
  try {
    const { videoPath, segments } = req.body;
    const inputPath = join(BASE_DIR, videoPath);
    const outputDir = join(BASE_DIR, 'edited');
    
    // Prevent directory traversal
    if (!inputPath.startsWith(BASE_DIR)) {
      return res.status(403).json({ error: 'Access denied' });
    }

    // Create output directory if it doesn't exist
    await fs.mkdir(outputDir, { recursive: true });

    const outputFileName = `edited_${Date.now()}_${videoPath.split('/').pop()}`;
    const outputPath = join(outputDir, outputFileName);

    // Create FFmpeg command
    let command = ffmpeg(inputPath);

    // Add segments filter
    const filterComplex = segments
      .map((segment, index) => {
        const startTime = segment.startTime;
        const endTime = segment.endTime;
        return `[0:v]trim=start=${startTime}:end=${endTime},setpts=PTS-STARTPTS[v${index}]; ` +
               `[0:a]atrim=start=${startTime}:end=${endTime},asetpts=PTS-STARTPTS[a${index}];`;
      })
      .join('');

    const concatFilter = segments
      .map((_, index) => `[v${index}][a${index}]`)
      .join('') +
      `concat=n=${segments.length}:v=1:a=1[outv][outa]`;

    command
      .complexFilter(filterComplex + concatFilter, ['outv', 'outa'])
      .map(outputPath);

    // Execute FFmpeg command
    command.on('end', () => {
      res.json({ 
        success: true, 
        output: outputFileName 
      });
    }).on('error', (err) => {
      console.error('Error processing video:', err);
      res.status(500).json({ 
        error: 'Failed to process video',
        details: err.message 
      });
    }).run();
  } catch (error) {
    console.error('Error processing video request:', error);
    res.status(500).json({ 
      error: 'Failed to process video request',
      details: error.message 
    });
  }
});

// Start server
app.listen(PORT, () => {
  console.log(`Server running on http://localhost:${PORT}`);
});