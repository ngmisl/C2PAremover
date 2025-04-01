[![Go](https://github.com/ngmisl/C2PAremover/actions/workflows/go.yml/badge.svg)](https://github.com/ngmisl/C2PAremover/actions/workflows/go.yml)

# C2PA Metadata Checker and Remover

A command-line tool to detect and remove Content Authenticity Initiative (CAI) metadata, also known as C2PA (Coalition for Content Provenance and Authenticity) metadata, from image files.

## What is C2PA Metadata?

C2PA (Coalition for Content Provenance and Authenticity) is a technical standard for providing provenance and history for digital content. While this can be useful for verifying content authenticity, it can also contain identifying information that some users may prefer to remove for privacy reasons.

This metadata is typically embedded in images as XMP (Extensible Metadata Platform) data in JPEG files or in iTXt chunks in PNG files.

## Features

- Detect C2PA metadata in JPEG and PNG files
- Two-tier removal approach:
  - Smart mode: Decode and re-encode the image (automatically strips most metadata)
  - Fallback mode: Custom JPEG segment parsing to precisely target C2PA data
- Preserve image quality with high-quality encoding (95% quality)
- Batch check directories of images for C2PA metadata
- Visual feedback with emoji indicators
- Size comparison between original and cleaned files
- Verification of cleaned files to ensure metadata was properly removed

## Installation

### Prerequisites

- Go 1.24.1 or higher

### Building from Source

1. Clone this repository:
```
git clone https://github.com/yourusername/c2paremover.git
cd c2paremover
```

2. Build the executable:
```
go build -o c2paremover
```

3. Optionally, install it to your system:
```
go install
```

## Usage

### Check a Single Image

```
./c2paremover check image.jpg
```

### Remove C2PA from an Image

```
./c2paremover remove image.jpg
```
This will create a new file with the `.cleaned.jpg` extension.

### Check a Directory of Images

```
./c2paremover check-dir /path/to/image/directory
```

## Example Output

When checking an image:
```
✓ No C2PA metadata found
```
or
```
⚠️  C2PA metadata detected
```

When removing metadata:
```
Removing C2PA metadata...
Removing C2PA metadata segment
✓ Cleaned file saved as image.jpg.cleaned.jpg (92.3% of original size)
✓ Verification: No C2PA metadata in cleaned file
```

When checking a directory:
```
✓ image1.jpg: No C2PA metadata
⚠️  image2.jpg: C2PA metadata detected
⚠️  image3.jpg: C2PA metadata detected
✓ image4.jpg: No C2PA metadata

Summary: Checked 4 images, found C2PA metadata in 2 images
```

## How It Works

### Detection Method

The tool checks for C2PA metadata using multiple indicators:
- C2PA namespace URI (`http://c2pa.org/`)
- C2PA manifest and claim tags
- CAI and related keywords
- Both JPEG APP1 segments and PNG text chunks

### Removal Methods

1. **Smart Mode**: First attempts to decode and re-encode the image using Go's standard image library. This automatically strips most metadata while preserving image quality.

2. **Fallback Mode**: If Smart Mode fails or doesn't remove the C2PA data, the tool switches to a more detailed approach that:
   - Parses the JPEG file structure segment-by-segment
   - Identifies APP1 segments containing XMP metadata
   - Analyzes the content for C2PA markers
   - Rebuilds the file without the C2PA segments

## License

[License](LICENSE)

## Contributing

Contributions, bug reports, and feature requests are welcome!

Made with love and vibes [Support the Project](https://fourzerofour.fkey.id)
