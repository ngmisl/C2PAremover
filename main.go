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
)

// JPEG markers
const (
	MARKER_SOI  = 0xFFD8 // Start of Image
	MARKER_APP1 = 0xFFE1 // APP1 marker for XMP/EXIF data
	MARKER_SOS  = 0xFFDA // Start of Scan
)

// CheckC2PA checks if an image has C2PA metadata
func CheckC2PA(data []byte) bool {
	// First check if this is a JPEG image
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		fmt.Println("Not a valid JPEG file")
		return false
	}

	// Check all APP1 segments
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
		
		// Check if it's an APP1 segment
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
		} else if markerType >= 0xE0 && markerType <= 0xEF {
			// Skip other APP segments
			length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
			pos += 2 + length
		} else {
			// Skip other markers
			pos += 2
		}
	}

	// Also check for PNG-specific C2PA metadata
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		// This is a PNG file, let's look for iTXt chunks with C2PA data
		if bytes.Contains(data, []byte("C2PA")) || 
		   bytes.Contains(data, []byte("c2pa")) ||
		   bytes.Contains(data, []byte("cai:")) {
			return true
		}
	}

	return false
}

// RemoveC2PA removes C2PA metadata from image
func RemoveC2PA(data []byte) ([]byte, error) {
	// First check if this is a JPEG image
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, fmt.Errorf("not a valid JPEG file")
	}

	// Try standard library reencoding method first
	img, format, err := image.Decode(bytes.NewReader(data))
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
				return removeC2PAFallback(data)
			}
		case "png":
			err = png.Encode(&buf, img)
			if err != nil {
				fmt.Println("Warning: PNG encoding failed, using fallback method")
				return removeC2PAFallback(data)
			}
		default:
			return removeC2PAFallback(data)
		}

		// Check if C2PA metadata is still present
		if !CheckC2PA(buf.Bytes()) {
			return buf.Bytes(), nil
		}
		
		fmt.Println("Warning: C2PA metadata persisted after standard reencoding, using fallback method")
	} else {
		fmt.Println("Warning: Image decoding failed, using fallback method")
	}

	// Fallback to custom JPEG segment parsing if standard reencoding fails or doesn't remove C2PA
	return removeC2PAFallback(data)
}

// Fallback method for removing C2PA metadata using custom JPEG segment parsing
func removeC2PAFallback(data []byte) ([]byte, error) {
	result := make([]byte, 0, len(data))
	result = append(result, 0xFF, 0xD8) // SOI marker
	
	pos := 2 // Skip SOI marker
	for pos < len(data)-4 {
		// Check for marker
		if data[pos] != 0xFF {
			pos++
			continue
		}

		markerType := data[pos+1]
		
		// If we've reached SOS, copy the rest of the file and exit
		if markerType == 0xDA {
			result = append(result, data[pos:]...)
			break
		}
		
		// Check if it's an APP1 segment
		if markerType == 0xE1 {
			// Get segment length (includes length bytes but not marker)
			length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
			if pos+2+length > len(data) {
				// Invalid length
				pos += 2
				continue
			}
			
			segmentData := data[pos+4:pos+2+length]
			
			// Check if it's XMP data with C2PA
			containsC2PA := false
			if bytes.HasPrefix(segmentData, []byte("http://ns.adobe.com/xap/1.0/")) {
				xmpString := string(segmentData)
				
				// Check for C2PA identifiers
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
			
			// Skip to next segment
			pos += 2 + length
		} else {
			// Keep other segments
			if markerType >= 0xE0 && markerType <= 0xEF {
				// APP segments
				length := int(binary.BigEndian.Uint16(data[pos+2:pos+4]))
				if pos+2+length > len(data) {
					// Invalid length
					pos += 2
					continue
				}
				result = append(result, data[pos:pos+2+length]...)
				pos += 2 + length
			} else {
				// Just copy the marker for now
				result = append(result, data[pos], data[pos+1])
				pos += 2
			}
		}
	}

	return result, nil
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
