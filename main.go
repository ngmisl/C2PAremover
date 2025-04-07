//go:build !wasmer
// +build !wasmer

// This file is built when the "wasmer" build tag is NOT specified.
// It contains the native implementation of the C2PA metadata remover.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// C2PA metadata markers
const (
	C2PA_NAMESPACE    = "http://c2pa.org/"
	C2PA_MANIFEST_TAG = "c2pa:manifest"
	C2PA_CLAIM_TAG    = "c2pa:claim"
	
	// JPEG specific markers
	MARKER_SOI  = 0xFFD8 // Start of Image
	MARKER_APP1 = 0xFFE1 // APP1 marker for XMP/EXIF data
	MARKER_APP11 = 0xFFEB // APP11 marker where C2PA also lives
	MARKER_SOS  = 0xFFDA // Start of Scan
)

// CheckC2PA checks if an image has C2PA metadata
func CheckC2PA(data []byte) bool {
	// Detect image format and use appropriate checker
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		// JPEG file
		return checkC2PAJPEG(data)
	} else if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		// PNG file
		return checkC2PAPNG(data)
	}
	
	// Unsupported format
	fmt.Println("Not a supported image format")
	return false
}

// checkC2PAJPEG checks if a JPEG image has C2PA metadata
func checkC2PAJPEG(data []byte) bool {
	// Check all APP1 and APP11 segments
	pos := 2 // Skip SOI marker
	for pos < len(data)-4 {
		// Check for marker
		if data[pos] != 0xFF {
			pos++
			continue
		}

		markerType := data[pos+1]
		
		// If we've reached SOS, we're done checking metadata segments
		if markerType == 0xDA {
			break
		}
		
		// Check if it's an APP1 segment with XMP data
		if markerType == 0xE1 {
			// Get segment length (includes length bytes but not marker)
			length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
			if pos+2+length > len(data) {
				// Invalid length
				pos += 2
				continue
			}
			
			segmentData := data[pos+4:pos+2+length]
			
			// Check if it's XMP data
			if bytes.HasPrefix(segmentData, []byte("http://ns.adobe.com/xap/1.0/")) {
				// Convert to string for easier regex matching
				xmpString := string(segmentData)
				
				// Check for C2PA namespace
				if strings.Contains(xmpString, C2PA_NAMESPACE) {
					return true
				}
				
				// Check for C2PA manifest or claim tags
				if strings.Contains(xmpString, C2PA_MANIFEST_TAG) || 
				   strings.Contains(xmpString, C2PA_CLAIM_TAG) {
					return true
				}
				
				// Use regex to check for C2PA related content
				c2paRegex := regexp.MustCompile(`(?i)c2pa|contentauthenticity|contentcredentials|cai`)
				if c2paRegex.MatchString(xmpString) {
					return true
				}
			}
			
			// Skip to next segment
			pos += 2 + length
		} else if markerType == 0xEB { // APP11 - where C2PA data can also be found
			// Get segment length
			length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
			if pos+2+length > len(data) {
				// Invalid length
				pos += 2
				continue
			}
			
			// APP11 segment can contain C2PA data directly
			return true
		} else if markerType >= 0xE0 && markerType <= 0xEF {
			// Skip other APP segments
			length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
			if pos+2+length > len(data) {
				// Invalid length
				pos += 2
				continue
			}
			pos += 2 + length
		} else {
			// Skip other markers
			pos += 2
		}
	}
	
	return false
}

// checkC2PAPNG checks if a PNG image has C2PA metadata
func checkC2PAPNG(data []byte) bool {
	// Check for C2PA related strings in the PNG data
	// Look for common C2PA identifiers in iTXt or tEXt chunks
	if bytes.Contains(data, []byte("C2PA")) || 
	   bytes.Contains(data, []byte("c2pa")) ||
	   bytes.Contains(data, []byte("cai:")) ||
	   bytes.Contains(data, []byte("contentauthenticity")) ||
	   bytes.Contains(data, []byte("contentcredentials")) {
		return true
	}
	
	// More detailed parsing could be added here
	// For simplicity and consistency with the WASM implementation
	// we'll use the simple content check approach
	
	return false
}

// RemoveC2PA removes C2PA metadata from image
func RemoveC2PA(data []byte) ([]byte, error) {
	// Detect image format
	var format string
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		format = "jpeg"
	} else if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		format = "png"
	} else {
		return nil, fmt.Errorf("unsupported image format")
	}

	// Try standard library reencoding method first
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		// Create a new buffer to store the cleaned image
		var buf bytes.Buffer

		// Re-encode the image without metadata based on format
		switch format {
		case "jpeg":
			// Use standard JPEG encoder with high quality
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
			if err != nil {
				fmt.Println("Warning: JPEG encoding failed, using fallback method")
				return removeC2PAFallbackJPEG(data)
			}
		case "png":
			err = png.Encode(&buf, img)
			if err != nil {
				fmt.Println("Warning: PNG encoding failed, using fallback method")
				return removeC2PAFallbackPNG(data)
			}
		default:
			return nil, fmt.Errorf("unsupported format: %s", format)
		}

		// Check if C2PA metadata is still present
		if !CheckC2PA(buf.Bytes()) {
			return buf.Bytes(), nil
		}
		
		fmt.Println("Warning: C2PA metadata persisted after standard reencoding, using fallback method")
	} else {
		fmt.Println("Warning: Image decoding failed, using fallback method")
	}

	// Fallback to custom segment parsing if standard reencoding fails or doesn't remove C2PA
	if format == "jpeg" {
		return removeC2PAFallbackJPEG(data)
	} else if format == "png" {
		return removeC2PAFallbackPNG(data)
	}
	
	return nil, fmt.Errorf("no suitable removal method for format: %s", format)
}

// Fallback method for removing C2PA metadata from JPEG using custom segment parsing
func removeC2PAFallbackJPEG(data []byte) ([]byte, error) {
	// First check if this is a JPEG image
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, fmt.Errorf("not a valid JPEG file")
	}
	
	// Parse using the SOI marker in first two bytes
	result := []byte{0xFF, 0xD8} // Start with SOI marker
	
	// For each segment, decide whether to keep it or discard it
	pos := 2 // Skip SOI marker
	foundSOS := false
	
	for pos < len(data)-1 {
		// Check for marker starting with 0xFF
		if data[pos] != 0xFF {
			// Skip unexpected bytes until we find the start of a marker
			// This makes the parser more robust against malformed JPEG files
			pos++
			continue
		}
		
		// Ensure there's enough data to read marker type
		if pos+1 >= len(data) {
			break // Reached end of data
		}
		
		markerType := data[pos+1]
		
		// If we've reached SOS, copy the rest of the file
		if markerType == 0xDA { // Start of Scan
			foundSOS = true
			// Add SOS marker and copy the rest of the file (image data)
			result = append(result, data[pos:]...)
			break
		}
		
		// If it's EOI, we've reached the end
		if markerType == 0xD9 { // End of Image
			result = append(result, data[pos:pos+2]...)
			break
		}
		
		// Handle markers that don't have length
		if (markerType >= 0xD0 && markerType <= 0xD7) || markerType == 0x01 {
			result = append(result, 0xFF, markerType)
			pos += 2
			continue
		}
		
		// All other markers should have a length field
		if pos+4 > len(data) {
			break // Not enough data for length
		}
		
		// Get segment length (includes length bytes but not marker)
		length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
		if length < 2 {
			// Invalid length, skip marker and continue
			pos += 2
			continue
		}
		
		// Make sure there's enough data for the full segment
		if pos+2+length > len(data) {
			// Not enough data, skip to end
			break
		}
		
		// Check if it's an APP1 or APP11 segment potentially containing C2PA data
		if markerType == 0xE1 { // APP1 marker
			containsC2PA := false
			
			// Only check XMP segments containing namespace
			segmentData := data[pos+4:pos+2+length]
			if bytes.HasPrefix(segmentData, []byte("http://ns.adobe.com/xap/1.0/")) {
				// Convert to string for easier string matching
				xmpString := string(segmentData)
				
				// Check for C2PA namespace or tags
				if strings.Contains(xmpString, C2PA_NAMESPACE) ||
				   strings.Contains(xmpString, C2PA_MANIFEST_TAG) || 
				   strings.Contains(xmpString, C2PA_CLAIM_TAG) {
					containsC2PA = true
				}
				
				// Also use regex for more comprehensive detection
				c2paRegex := regexp.MustCompile(`(?i)c2pa|contentauthenticity|contentcredentials|cai`)
				if c2paRegex.MatchString(xmpString) {
					containsC2PA = true
				}
			}
			
			// Only keep segment if it doesn't contain C2PA data
			if !containsC2PA {
				result = append(result, data[pos:pos+2+length]...)
			} else {
				fmt.Println("Removing C2PA metadata segment")
			}
		} else if markerType == 0xEB { // APP11 marker, which often contains C2PA data
			// Skip this segment as it might contain C2PA data
			fmt.Println("Removing APP11 segment that might contain C2PA metadata")
		} else {
			// Keep other segments
			result = append(result, data[pos:pos+2+length]...)
		}
		
		// Move to next segment
		pos += 2 + length
	}
	
	// If we didn't find the SOS marker, make sure we have an EOI marker at the end
	if !foundSOS && !bytes.HasSuffix(result, []byte{0xFF, 0xD9}) {
		result = append(result, 0xFF, 0xD9) // Add EOI marker to ensure valid JPEG
	}
	
	return result, nil
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
			fmt.Printf("PNG chunk truncated (%s, length %d)\n", chunkType, length)
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
	
	return chunks
}

// removeC2PAFallbackPNG removes C2PA metadata from PNG by selectively copying non-C2PA chunks
func removeC2PAFallbackPNG(data []byte) ([]byte, error) {
	fmt.Println("Using fallback PNG chunk removal method")
	
	// Extract all PNG chunks
	chunks := extractPNGChunks(data)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("failed to parse PNG chunks")
	}
	
	// Create a new buffer for the cleaned PNG
	buf := new(bytes.Buffer)
	
	// Write PNG signature
	_, _ = buf.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	
	// Track if we've removed any chunks
	removed := false
	
	// Copy all chunks except those containing C2PA data
	for i, chunk := range chunks {
		// Check text chunks for C2PA content
		isC2PAChunk := false
		if chunk.chunkType == "iTXt" || chunk.chunkType == "tEXt" {
			chunkData := string(chunk.data)
			if strings.Contains(strings.ToLower(chunkData), "c2pa") || 
			   strings.Contains(strings.ToLower(chunkData), "contentauthenticity") ||
			   strings.Contains(strings.ToLower(chunkData), "cai:") {
				fmt.Printf("Removing C2PA chunk #%d (type: %s)\n", i, chunk.chunkType)
				isC2PAChunk = true
				removed = true
			}
		}
		
		// Skip C2PA chunks
		if isC2PAChunk {
			continue
		}
		
		// Write chunk length (4 bytes)
		lengthBytes := []byte{
			byte(chunk.length >> 24),
			byte(chunk.length >> 16),
			byte(chunk.length >> 8),
			byte(chunk.length),
		}
		_, _ = buf.Write(lengthBytes)
		
		// Write chunk type (4 bytes)
		_, _ = buf.Write([]byte(chunk.chunkType))
		
		// Write chunk data
		_, _ = buf.Write(chunk.data)
		
		// Write CRC (4 bytes)
		crcBytes := []byte{
			byte(chunk.crc >> 24),
			byte(chunk.crc >> 16),
			byte(chunk.crc >> 8),
			byte(chunk.crc),
		}
		_, _ = buf.Write(crcBytes)
	}
	
	if !removed {
		fmt.Println("No C2PA chunks found to remove in PNG")
		return data, fmt.Errorf("no C2PA chunks found to remove")
	}
	
	cleanedData := buf.Bytes()
	fmt.Printf("PNG fallback removal finished (%d bytes)\n", len(cleanedData))
	
	// Verify removal was successful
	if CheckC2PA(cleanedData) {
		fmt.Println("Error: PNG fallback removal failed verification check")
		return data, fmt.Errorf("PNG fallback removal failed verification check")
	}
	
	fmt.Println("PNG fallback removal verified successfully")
	return cleanedData, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: c2paremover [check|remove] <image_file>")
		fmt.Println("Examples:")
		fmt.Println("  c2paremover check image.jpg")
		fmt.Println("  c2paremover remove image.jpg")
		fmt.Println("  c2paremover check-dir directory")
		return
	}

	mode := os.Args[1]

	if mode == "check-dir" {
		if len(os.Args) < 3 {
			fmt.Println("Please specify a directory")
			return
		}
		
		dirPath := os.Args[2]
		checkDirectory(dirPath)
		return
	}

	if len(os.Args) < 3 {
		fmt.Println("Please specify an image file")
		return
	}
	
	filePath := os.Args[2]

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	switch mode {
	case "check":
		if CheckC2PA(data) {
			fmt.Println("⚠️  C2PA metadata detected")
		} else {
			fmt.Println("✓ No C2PA metadata found")
		}
	case "remove":
		if !CheckC2PA(data) {
			fmt.Println("No C2PA metadata found, no changes needed")
			return
		}
		
		fmt.Println("Removing C2PA metadata...")
		newData, err := RemoveC2PA(data)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		
		if len(newData) == len(data) && bytes.Equal(newData, data) {
			fmt.Println("No changes made")
		} else {
			cleanPath := filePath + ".cleaned" + filepath.Ext(filePath)
			err = os.WriteFile(cleanPath, newData, 0644)
			if err != nil {
				fmt.Println("Error saving file:", err)
			} else {
				fmt.Printf("✓ Cleaned file saved as %s (%.1f%% of original size)\n", 
					cleanPath, float64(len(newData))/float64(len(data))*100)
				
				// Verify the cleaned file
				cleanData, _ := os.ReadFile(cleanPath)
				if CheckC2PA(cleanData) {
					fmt.Println("⚠️  Warning: C2PA metadata still detected in cleaned file")
				} else {
					fmt.Println("✓ Verification: No C2PA metadata in cleaned file")
				}
			}
		}
	default:
		fmt.Println("Invalid mode. Use 'check', 'remove', or 'check-dir'")
	}
}

// Check all image files in a directory for C2PA metadata
func checkDirectory(dirPath string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return
	}

	imagesChecked := 0
	imagesWithC2PA := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Println("Error reading file:", filePath, err)
			continue
		}

		imagesChecked++
		hasC2PA := CheckC2PA(data)
		if hasC2PA {
			imagesWithC2PA++
			fmt.Printf("⚠️  %s: C2PA metadata detected\n", file.Name())
		} else {
			fmt.Printf("✓ %s: No C2PA metadata\n", file.Name())
		}
	}

	fmt.Printf("\nSummary: Checked %d images, found C2PA metadata in %d images\n", 
		imagesChecked, imagesWithC2PA)
}
