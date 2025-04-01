package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestCheckC2PA tests the C2PA detection functionality
func TestCheckC2PA(t *testing.T) {
	tests := []struct {
		name     string
		testFile string
		expected bool
	}{
		{
			name:     "Empty data",
			testFile: nil,
			expected: false,
		},
		{
			name:     "Non-JPEG data",
			testFile: []byte("This is not a JPEG file"),
			expected: false,
		},
		{
			name:     "Minimal JPEG without C2PA",
			testFile: createMinimalJPEG(false),
			expected: false,
		},
		{
			name:     "Minimal JPEG with C2PA",
			testFile: createMinimalJPEG(true),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var data []byte
			if tc.testFile == nil {
				data = []byte{}
			} else if tc.testFile != nil && len(tc.testFile) > 0 && tc.testFile[0] == 0 {
				// If the first byte is 0, interpret as raw data
				data = tc.testFile
			} else {
				data = tc.testFile
			}

			result := CheckC2PA(data)
			if result != tc.expected {
				t.Errorf("CheckC2PA() = %v, want %v", result, tc.expected)
			}
		})
	}
}

// TestRemoveC2PA tests the C2PA removal functionality
func TestRemoveC2PA(t *testing.T) {
	tests := []struct {
		name          string
		testFile      []byte
		shouldChange  bool
		shouldSucceed bool
	}{
		{
			name:          "Empty data",
			testFile:      []byte{},
			shouldChange:  false,
			shouldSucceed: false,
		},
		{
			name:          "Non-JPEG data",
			testFile:      []byte("This is not a JPEG file"),
			shouldChange:  false,
			shouldSucceed: false,
		},
		{
			name:          "Minimal JPEG without C2PA",
			testFile:      createMinimalJPEG(false),
			shouldChange:  false,
			shouldSucceed: true,
		},
		{
			name:          "Minimal JPEG with C2PA",
			testFile:      createMinimalJPEG(true),
			shouldChange:  true,
			shouldSucceed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalData := tc.testFile
			newData, err := RemoveC2PA(originalData)

			if tc.shouldSucceed && err != nil {
				t.Errorf("RemoveC2PA() failed with error: %v", err)
				return
			}

			if !tc.shouldSucceed && err == nil {
				t.Errorf("RemoveC2PA() should have failed but didn't")
				return
			}

			if tc.shouldSucceed {
				if tc.shouldChange && bytes.Equal(originalData, newData) {
					t.Errorf("RemoveC2PA() didn't change the data when it should have")
				}

				if !tc.shouldChange && !bytes.Equal(originalData, newData) {
					t.Errorf("RemoveC2PA() changed the data when it shouldn't have")
				}

				// If we expected to change the data, verify that the C2PA is removed
				if tc.shouldChange && CheckC2PA(newData) {
					t.Errorf("RemoveC2PA() didn't remove C2PA metadata")
				}

				// If we have valid JPEG data, make sure it still starts with JPEG marker
				if len(newData) >= 2 {
					if newData[0] != 0xFF || newData[1] != 0xD8 {
						t.Errorf("RemoveC2PA() result is not a valid JPEG (doesn't start with SOI marker)")
					}
				}
			}
		})
	}
}

// TestRemoveC2PAIntegration performs integration tests with real files if available
func TestRemoveC2PAIntegration(t *testing.T) {
	// Create test directory if it doesn't exist
	testDir := "testdata"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Try to use existing test files if available
	matches, err := filepath.Glob(filepath.Join(testDir, "*.jpg"))
	if err != nil {
		t.Logf("Error searching for test files: %v", err)
		matches = []string{}
	}

	// If we don't have real test files, create mock ones
	if len(matches) == 0 {
		t.Log("No test images found, creating mock test files")
		
		// Create a mock image without C2PA
		noC2PAPath := filepath.Join(testDir, "no_c2pa.jpg")
		if err := os.WriteFile(noC2PAPath, createMinimalJPEG(false), 0644); err != nil {
			t.Fatalf("Failed to create mock test file: %v", err)
		}
		matches = append(matches, noC2PAPath)
		
		// Create a mock image with C2PA
		withC2PAPath := filepath.Join(testDir, "with_c2pa.jpg")
		if err := os.WriteFile(withC2PAPath, createMinimalJPEG(true), 0644); err != nil {
			t.Fatalf("Failed to create mock test file: %v", err)
		}
		matches = append(matches, withC2PAPath)
	}

	// Test each image file
	for _, filePath := range matches {
		t.Run(filepath.Base(filePath), func(t *testing.T) {
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read test file %s: %v", filePath, err)
			}

			// Check if the file has C2PA metadata
			hasC2PA := CheckC2PA(data)
			t.Logf("File %s has C2PA: %v", filePath, hasC2PA)

			// Try to remove C2PA metadata
			newData, err := RemoveC2PA(data)
			if err != nil {
				t.Fatalf("RemoveC2PA() failed: %v", err)
			}

			// Check if C2PA was removed (or was never there)
			if CheckC2PA(newData) {
				t.Errorf("C2PA metadata still detected after removal")
			}

			// Save the cleaned file for inspection
			cleanedPath := filePath + ".test.cleaned" + filepath.Ext(filePath)
			if err := os.WriteFile(cleanedPath, newData, 0644); err != nil {
				t.Fatalf("Failed to write cleaned test file: %v", err)
			}
			t.Logf("Cleaned file saved as %s", cleanedPath)
		})
	}
}

// Helper function to create a minimal valid JPEG file for testing
func createMinimalJPEG(withC2PA bool) []byte {
	// Start with SOI marker
	data := []byte{0xFF, 0xD8}

	// Add APP0 (JFIF) marker
	jfif := []byte{
		0xFF, 0xE0,                   // APP0 marker
		0x00, 0x10,                   // Length (16 bytes)
		0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
		0x01, 0x01,                   // Version 1.1
		0x00,                         // Units (0 = none)
		0x00, 0x01, 0x00, 0x01,       // Density (1x1)
		0x00, 0x00,                   // Thumbnail (none)
	}
	data = append(data, jfif...)

	// If withC2PA, add a mock APP1 segment with C2PA content
	if withC2PA {
		// Create a simplified XMP data block with C2PA namespace
		xmp := "http://ns.adobe.com/xap/1.0/ <x:xmpmeta xmlns:x='adobe:ns:meta/'><rdf:RDF xmlns:rdf='http://www.w3.org/1999/02/22-rdf-syntax-ns#'><rdf:Description rdf:about='' xmlns:c2pa='http://c2pa.org/'>C2PA test metadata</rdf:Description></rdf:RDF></x:xmpmeta>"
		xmpBytes := []byte(xmp)
		
		// APP1 header (marker + length)
		app1Header := []byte{
			0xFF, 0xE1, // APP1 marker
			byte((len(xmpBytes) + 2) >> 8), byte((len(xmpBytes) + 2) & 0xFF), // Length including length bytes
		}
		
		data = append(data, app1Header...)
		data = append(data, xmpBytes...)
	}

	// Add minimal SOS marker to make it a valid JPEG
	sos := []byte{
		0xFF, 0xDA,       // SOS marker
		0x00, 0x08,       // Length (8 bytes)
		0x01,             // 1 component
		0x01, 0x00,       // Component ID and huffman table
		0x00, 0x3F, 0x00, // Start of spectral, end of spectral, approximation bit
	}
	data = append(data, sos...)

	// Add some dummy image data
	data = append(data, []byte{0x00, 0xFF, 0xD9}...) // Random data + EOI marker

	return data
}
