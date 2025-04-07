//go:build wasmer

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"
)

// Constants for C2PA markers (remains the same)
const (
	c2paMarkerJPEG = 0xEB // APP11 marker for C2PA in JPEG
	c2paNamespace  = "http://ns.adobe.com/xap/1.0/"
	c2paManifest   = "c2pa.manifest"
	
	// Debug mode flag - set to false to disable verbose logging in production
	debugMode = true
)

// debugLog outputs debug messages only when debugMode is true
func debugLog(format string, args ...interface{}) {
	if debugMode {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

func main() {
	// Read all data from standard input
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Debug: Print input size and start
	debugLog("Debug WASM: Received %d bytes from stdin\n", len(inputData))
	if len(inputData) > 10 {
		debugLog("Debug WASM: Input starts with: %X\n", inputData[:10])
	} else if len(inputData) > 0 {
		debugLog("Debug WASM: Input starts with: %X\n", inputData)
	}

	if len(inputData) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No input data received")
		os.Exit(1)
	}

	// Check if the input data has C2PA metadata
	debugLog("Debug WASM: Calling CheckC2PA...\n")
	hasC2PA := CheckC2PA(inputData)
	debugLog("Debug WASM: CheckC2PA returned: %v\n", hasC2PA)

	if !hasC2PA {
		// If no C2PA, output the original data and exit
		_, err = os.Stdout.Write(inputData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing original data to output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Input does not contain C2PA metadata.") // This is the message we saw
		os.Exit(0)
	}

	// If C2PA is present, attempt to remove it
	fmt.Fprintln(os.Stderr, "C2PA metadata detected, attempting removal...")
	cleanedData, err := RemoveC2PA(inputData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing C2PA metadata: %v\n", err)
		// Output original data on failure
		_, writeErr := os.Stdout.Write(inputData)
		if writeErr != nil {
			fmt.Fprintf(os.Stderr, "Error writing original data after removal failure: %v\n", writeErr)
		}
		os.Exit(1)
	}

	// Write the cleaned data to standard output
	_, err = os.Stdout.Write(cleanedData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing cleaned data to output: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "C2PA metadata removed successfully.")
	os.Exit(0)
}

// CheckC2PA checks if an image (JPEG or PNG) has C2PA metadata
// (Function remains mostly the same, might need minor adjustments if format detection relied on filename)
func CheckC2PA(data []byte) bool {
	// Try JPEG first
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		debugLog("Debug WASM: Detected JPEG prefix")
		return checkC2PAJPEG(data)
	}
	// Try PNG
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		debugLog("Debug WASM: Detected PNG prefix")
		return checkC2PAPNG(data)
	}
	// Add checks for other formats if needed
	debugLog("Debug WASM: Unknown format prefix")
	return false
}

func checkC2PAJPEG(data []byte) bool {
	segments := parseJPEG(data)
	if segments == nil {
		debugLog("Debug WASM: parseJPEG returned nil")
		return false
	}
	
	debugLog("Debug WASM: Found %d JPEG segments\n", len(segments))
	
	for i, seg := range segments {
		// Limit excessive logging for large files
		if i < 10 || i > len(segments)-5 { // Log first 10 and last 5 segments
            debugLog("Debug WASM: Checking segment %d: Marker=0x%X Length=%d\n", i, seg.Marker, seg.Length)
        }
		
		// Check for APP11 (0xEB) which is where C2PA typically lives in JPEG
		if seg.Marker == 0xEB { // APP11 (0xFFEB in the JPEG file)
			debugLog("Debug WASM: Found C2PA potential marker (APP11)")
			// Optional: deeper inspection of the segment data here to confirm it's C2PA
			return true
		}
		
		// Check APP1 (0xE1) for XMP containing C2PA namespace or manifest
		if seg.Marker == 0xE1 { // APP1 (0xFFE1 in the JPEG file)
			// Only log check if segment data isn't huge
            if seg.Length < 1024 {
                debugLog("Debug WASM: Checking APP1 segment (len %d) for C2PA strings\n", seg.Length)
            } else {
                debugLog("Debug WASM: Checking large APP1 segment (len %d) for C2PA strings\n", seg.Length)
            }
			
			if bytes.Contains(seg.Data, []byte(c2paNamespace)) || bytes.Contains(seg.Data, []byte(c2paManifest)) {
				debugLog("Debug WASM: Found C2PA namespace or manifest in APP1")
				return true
			}
		}
	}
	
	debugLog("Debug WASM: No C2PA markers found in JPEG segments")
	return false
}

// checkC2PAPNG checks for C2PA in PNG data
func checkC2PAPNG(data []byte) bool {
	debugLog("Debug WASM: Checking PNG for C2PA metadata")
	
	// Verify PNG signature
	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		debugLog("Debug WASM: Invalid PNG signature")
		return false
	}
	
	// Check for C2PA related strings in the PNG data
	// Look for common C2PA identifiers in iTXt chunks
	if bytes.Contains(data, []byte("C2PA")) || 
	   bytes.Contains(data, []byte("c2pa")) ||
	   bytes.Contains(data, []byte("cai:")) ||
	   bytes.Contains(data, []byte("contentauthenticity")) ||
	   bytes.Contains(data, []byte("contentcredentials")) {
		debugLog("Debug WASM: Found C2PA identifier in PNG data")
		return true
	}
	
	// Additional checks for PNG iTXt chunks with C2PA data
	// Search for iTXt chunk signature followed by C2PA keywords
	chunks := extractPNGChunks(data)
	for _, chunk := range chunks {
		if chunk.chunkType == "iTXt" || chunk.chunkType == "tEXt" {
			chunkData := string(chunk.data)
			if strings.Contains(chunkData, "c2pa") || 
			   strings.Contains(chunkData, "C2PA") ||
			   strings.Contains(chunkData, "contentauthenticity") {
				debugLog("Debug WASM: Found C2PA in PNG text chunk")
				return true
			}
		}
	}
	
	debugLog("Debug WASM: No C2PA found in PNG")
	return false
}

// PNGChunk represents a chunk in a PNG file
type pngChunk struct {
	length    uint32
	chunkType string
	data      []byte
	crc       uint32
}

// extractPNGChunks extracts chunks from PNG data
func extractPNGChunks(data []byte) []pngChunk {
	var chunks []pngChunk
	
	// Skip the PNG signature (8 bytes)
	pos := 8
	
	for pos+12 <= len(data) { // Minimum chunk size: 4 (length) + 4 (type) + 0 (data) + 4 (CRC)
		// Read chunk length (4 bytes, big-endian)
		length := uint32(data[pos])<<24 | uint32(data[pos+1])<<16 | uint32(data[pos+2])<<8 | uint32(data[pos+3])
		pos += 4
		
		// Read chunk type (4 bytes)
		chunkType := string(data[pos:pos+4])
		pos += 4
		
		// Check if there's enough data for the chunk
		if pos+int(length)+4 > len(data) {
			debugLog("Debug WASM: PNG chunk truncated (%s, length %d)\n", chunkType, length)
			break
		}
		
		// Read chunk data
		chunkData := data[pos:pos+int(length)]
		pos += int(length)
		
		// Read CRC (4 bytes)
		crc := uint32(data[pos])<<24 | uint32(data[pos+1])<<16 | uint32(data[pos+2])<<8 | uint32(data[pos+3])
		pos += 4
		
		chunks = append(chunks, pngChunk{
			length:    length,
			chunkType: chunkType,
			data:      chunkData,
			crc:       crc,
		})
		
		// Break if we've reached the IEND chunk
		if chunkType == "IEND" {
			break
		}
	}
	
	debugLog("Debug WASM: Extracted %d PNG chunks\n", len(chunks))
	return chunks
}

// RemoveC2PA attempts to remove C2PA metadata
// (Function remains mostly the same, adapt logging)
func RemoveC2PA(data []byte) ([]byte, error) {
	debugLog("Debug WASM: Entering RemoveC2PA")
	format, err := detectImageFormat(data)
	if err != nil {
		return nil, fmt.Errorf("unsupported or invalid image format: %v", err)
	}
	debugLog("Debug WASM: Detected format: %s\n", format)

	// 1. Smart Mode: Try decoding and re-encoding using standard library
	debugLog("Debug WASM: Attempting smart mode (decode/re-encode)")
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		buf := new(bytes.Buffer)
		switch format {
		case "jpeg":
			debugLog("Debug WASM: Smart mode - Encoding JPEG")
			err = jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}) // Keep decent quality
		case "png":
			debugLog("Debug WASM: Smart mode - Encoding PNG")
			err = png.Encode(buf, img)
		default:
			debugLog("Debug WASM: Smart mode - Unsupported format %s for re-encoding\n", format)
			return nil, fmt.Errorf("re-encoding not supported for format: %s", format)
		}

		if err == nil {
			cleanedData := buf.Bytes()
			debugLog("Debug WASM: Smart mode successful. Verifying removal (%d bytes output)...\n", len(cleanedData))
			// Optional: Verify removal if needed by checking cleanedData again
			if !CheckC2PA(cleanedData) {
				debugLog("Smart mode removal verified successfully.")
				return cleanedData, nil
			} else {
				debugLog("Warning: Smart mode re-encoding did not remove C2PA (verification failed). Trying fallback...")
			}
		} else {
			debugLog("Warning: Smart mode re-encoding failed: %v. Trying fallback...\n", err)
		}
	} else {
		debugLog("Debug WASM: Image decode failed for smart mode: %v. Proceeding to fallback...\n", err)
	}

	// 2. Fallback Mode (JPEG specific for now)
	if format == "jpeg" {
		debugLog("Debug WASM: Using fallback JPEG segment removal.")
		segments := parseJPEG(data)
		buf := new(bytes.Buffer)
		_, _ = buf.Write([]byte{0xFF, 0xD8}) // SOI

		removed := false
		for i, seg := range segments {
			if seg.Marker == c2paMarkerJPEG || (seg.Marker == 0xE1 && (bytes.Contains(seg.Data, []byte(c2paNamespace)) || bytes.Contains(seg.Data, []byte(c2paManifest)))) {
				debugLog("Debug WASM: Fallback: Removing segment %d (Marker=0x%X)\n", i, seg.Marker)
				removed = true
				continue // Skip writing this segment
			}
			// Write segment if not removed
			_, _ = buf.Write([]byte{0xFF, byte(seg.Marker)}) // Write marker
			if seg.Length > 0 {                               // Marker length includes the 2 bytes for length itself
				lenBytes := []byte{byte(seg.Length >> 8), byte(seg.Length & 0xFF)}
				_, _ = buf.Write(lenBytes) // Write length
				_, _ = buf.Write(seg.Data) // Write data
			}
		}

		// Need to ensure EOI marker is present if it was in the original segments
		foundEOI := false
		for _, seg := range segments {
			if seg.Marker == 0xD9 { // EOI
				foundEOI = true
				break
			}
		}
		if foundEOI {
			_, _ = buf.Write([]byte{0xFF, 0xD9}) // EOI
			debugLog("Debug WASM: Fallback: Appended EOI marker.")
		} else {
             // If original segments didn't have EOI, maybe it's truncated? Add it just in case.
             // Cautious approach: only add if it was missing in the parsed segments. Many JPEGs omit it.
            debugLog("Warning: Original JPEG did not contain EOI marker (0xFFD9). Not adding one.")
        }

		if !removed {
			debugLog("Warning: Fallback mode did not find specific C2PA markers to remove.")
			// Return original data if nothing was actually removed by fallback
            // to avoid unnecessary modification.
            return data, fmt.Errorf("fallback mode found no C2PA markers to remove")
		}
        
        cleanedData := buf.Bytes()
		debugLog("Debug WASM: Fallback removal finished (%d bytes output). Verifying...\n", len(cleanedData))
        // Final verification
        if CheckC2PA(cleanedData) {
			 debugLog("Error: Fallback removal failed verification check.")
             return data, fmt.Errorf("fallback removal failed verification check")
        }
		debugLog("Debug WASM: Fallback removal verified.")
		return cleanedData, nil
	}

	// If fallback is not applicable (e.g., PNG and smart mode failed)
	debugLog("Debug WASM: Failed to remove C2PA (smart mode failed, no fallback for %s)\n", format)
	return nil, fmt.Errorf("failed to remove C2PA metadata (smart mode failed, no fallback for %s)", format)
}

// detectImageFormat detects if data is JPEG or PNG
// (Function remains the same)
func detectImageFormat(data []byte) (string, error) {
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "jpeg", nil
	}
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "png", nil
	}
	return "", fmt.Errorf("unknown image format")
}

// jpegSegment represents a segment in a JPEG file
// (Struct remains the same)
type jpegSegment struct {
	Marker int
	Length int // Length of the data payload (doesn't include marker or length bytes)
	Data   []byte
}

// parseJPEG parses JPEG segments
func parseJPEG(data []byte) []jpegSegment {
	var segments []jpegSegment
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 { // Check for SOI
		debugLog("Debug WASM: parseJPEG failed SOI check")
		return nil
	}
	// debugLog("Debug WASM: parseJPEG started") // Reduce noise
	pos := 2
	segmentCount := 0
	for pos < len(data)-1 {
		if data[pos] != 0xFF {
			// Skip non-FF bytes until we find the start of a marker or run out of data
            debugLog("Debug WASM: parseJPEG skipping unexpected byte 0x%X at pos %d\n", data[pos], pos)
            pos++
            continue
		}

		// Found 0xFF, check the next byte for the marker type
		if pos+1 >= len(data) {
			debugLog("Debug WASM: parseJPEG found 0xFF at end of data (pos %d)\n", pos)
            break // Reached end of data after finding 0xFF
        }
		marker := int(data[pos+1])
		pos += 2
		segmentCount++

		// Markers without payload length (RSTm, EOI, etc.)
        // Note: We handle EOI explicitly to break the loop.
        if (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
            segments = append(segments, jpegSegment{Marker: marker, Length: 0})
            continue // Move to the next marker search
		}
        
        // SOS marker (Start of Scan) - Stop parsing segments, rest is image data
        if marker == 0xDA {
             segments = append(segments, jpegSegment{Marker: marker, Length: 0})
             debugLog("Debug WASM: parseJPEG found SOS, stopping segment parse.")
             break
        }

        // EOI marker (End of Image)
        if marker == 0xD9 {
            segments = append(segments, jpegSegment{Marker: marker, Length: 0})
            debugLog("Debug WASM: parseJPEG found EOI, stopping parse.")
            break // Stop parsing after EOI
        }

		// All other markers should have a length field
		if pos+2 > len(data) {
			debugLog("Debug WASM: parseJPEG not enough data for length at pos %d (marker 0x%X)\n", pos, marker)
			break
		}

		length := int(data[pos])<<8 | int(data[pos+1])
		if length < 2 { // Length includes the 2 length bytes, so must be >= 2
             debugLog("Debug WASM: parseJPEG invalid length %d for marker 0x%X at pos %d\n", length, marker, pos)
             // Attempt to skip marker and continue searching? Risky.
             // Let's break for now, but a more robust parser might try to recover.
             break
        }
		payloadLength := length - 2
		pos += 2

		if pos+payloadLength > len(data) {
			debugLog("Debug WASM: parseJPEG not enough data for payload (%d bytes) for marker 0x%X at pos %d\n", payloadLength, marker, pos)
			break
		}

		segmentData := data[pos : pos+payloadLength]
		segments = append(segments, jpegSegment{Marker: marker, Length: length, Data: segmentData})
		pos += payloadLength
	}
	debugLog("Debug WASM: parseJPEG finished, parsed %d segments (stopped at pos %d)\n", len(segments), pos)
	return segments
}
