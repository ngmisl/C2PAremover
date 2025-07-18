[![Go](https://github.com/ngmisl/C2PAremover/actions/workflows/go.yml/badge.svg)](https://github.com/ngmisl/C2PAremover/actions/workflows/go.yml) [![CodeQL](https://github.com/ngmisl/C2PAremover/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/ngmisl/C2PAremover/actions/workflows/github-code-scanning/codeql) [![Build Farcaster Page](https://github.com/ngmisl/C2PAremover/actions/workflows/gh-frame.yml/badge.svg)](https://github.com/ngmisl/C2PAremover/actions/workflows/gh-frame.yml) [![Vibecondom Security Scan](https://github.com/ngmisl/C2PAremover/actions/workflows/security-scan.yaml/badge.svg)](https://github.com/ngmisl/C2PAremover/actions/workflows/security-scan.yaml)

# C2PA Metadata Remover

A lightweight tool for detecting and removing Content Authenticity Initiative (C2PA) metadata from image files. Available as both a CLI tool and a WebAssembly module.

## What is C2PA?

C2PA (Coalition for Content Provenance and Authenticity) is a metadata standard used to track the origin and edit history of media content. While it serves legitimate purposes in combating misinformation and deepfakes, it also raises privacy concerns as it can contain identifiable information about the device that created an image and its user.

## Features

- Detects presence of C2PA metadata in JPEG and PNG files
- Cleanly removes C2PA metadata while preserving image quality
- Provides robust fallback methods for both JPEG and PNG files
- Available in two formats:
  - Native Go CLI tool
  - WebAssembly module (via Wasmer)
- Doesn't require external dependencies

## Installation

### CLI Tool

#### From Source

```bash
# Requires Go 1.24.2 or later
git clone https://github.com/ngmisl/C2PAremover.git
cd C2PAremover
go build -o c2paremover .
```

#### Using Make

The project includes a Makefile to simplify building:

```bash
# Build native binary
make build

# Build WebAssembly binary
make wasm

# Build both
make all

# Install to system (requires sudo for /usr/local/bin)
sudo make install
```

### WebAssembly Module

```bash
# Install using Wasmer
wasmer install metaend/c2paremover

# Or run directly from Wasmer.io registry
wasmer run metaend/c2paremover@0.1.5
```

## Usage

### CLI Tool

```bash
# Check and remove C2PA metadata from a single file
c2paremover input.jpg output.jpg

# Process multiple files
c2paremover input1.jpg output1.jpg input2.png output2.png

# Check directory (creates cleaned copies with "_clean" suffix)
c2paremover -d /path/to/directory

# Check if an image has C2PA metadata without removing it
c2paremover check input.jpg
```

### WebAssembly Module

The WASM module reads from stdin and writes to stdout:

```bash
# Process a single file
cat input.jpg | wasmer run c2paremover > cleaned.jpg

# Process Adobe test file with C2PA metadata
cat adobe-20220124-CAICA.jpg | wasmer run metaend/c2paremover > cleaned.jpg

# Process and chain with other tools
cat input.jpg | wasmer run c2paremover | convert - -resize 800x600 output.jpg
```

#### Why Wasmer?

The WebAssembly version offers several advantages:

- **Cross-platform compatibility**: Run the same binary on any OS (Windows, macOS, Linux)
- **No installation required**: Just use the Wasmer CLI to run directly from the registry
- **Sandboxed execution**: Enhanced security through WebAssembly's isolation
- **Fast performance**: Near-native execution speed
- **Easy distribution**: Share a single link that works everywhere
- **Seamless pipelines**: Perfect for integration with other command-line tools

## Build Options

### Standard CLI Build

```bash
go build .
```

### WebAssembly Build

```bash
GOOS=wasip1 GOARCH=wasm go build -o c2paremover.wasm -tags=wasmer .
```

### Using Make

```bash
# Build all targets
make all
```

## How It Works

The tool performs the following operations:

1. Detects the image format (JPEG or PNG)
2. Parses the file structure to identify C2PA metadata
   - For JPEG: Checks for APP11 (0xEB) segments and APP1 (XMP) containing C2PA namespaces
   - For PNG: Checks for text chunks (iTXt and tEXt) containing C2PA references
3. When removing metadata:
   - Primarily attempts a "smart mode" by re-encoding the decoded image
   - If smart mode fails or doesn't fully remove C2PA:
     - For JPEGs: Uses a fallback approach that selectively copies segments, skipping C2PA-related ones
     - For PNGs: Uses a fallback that parses and reconstructs the file, omitting C2PA-related chunks

## Recent Improvements

- Enhanced PNG handling with dedicated fallback removal methods
- Unified detection and removal logic between native and WASM implementations
- Improved error handling and recovery from malformed image data
- Added conditional debug logging for better performance in production
- Build optimizations with Makefile support

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
