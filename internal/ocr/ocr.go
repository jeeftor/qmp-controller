package ocr

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"

	"github.com/spakin/netpbm"

	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeeftor/qmp-controller/internal/logging"
)

// CharacterBitmap represents a bitmap of a single character
type CharacterBitmap struct {
	Width  int             `json:"width"`
	Height int             `json:"height"`
	Data   [][]bool        `json:"data"`
	Colors [][]color.Color `json:"colors,omitempty"` // Color information for each pixel
	Char   string          `json:"char,omitempty"`   // The character this bitmap represents (for training)
}

// OCRResult represents the result of OCR processing
type OCRResult struct {
	Width       int               `json:"width"`       // Width in characters
	Height      int               `json:"height"`      // Height in characters
	Text        []string          `json:"text"`        // Recognized text as lines
	CharBitmaps []CharacterBitmap `json:"charBitmaps"` // Character bitmaps (for debug/training)
}

// TrainingData represents OCR training data
type TrainingData struct {
	BitmapMap map[string]string `json:"bitmapMap"` // Map of hex bitmap to character
}

// ProcessScreenshot processes a screenshot for OCR
func ProcessScreenshot(screenshotPath string, width, height int, debug bool) (*OCRResult, error) {
	return ProcessScreenshotWithTrainingData(screenshotPath, "", width, height, debug)
}

// decodeImageFile opens and decodes an image file using netpbm first, then standard decoders
func decodeImageFile(screenshotPath string) (image.Image, error) {
	file, err := os.Open(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open screenshot: %v", err)
	}
	defer file.Close()

	// Try to decode the image using netpbm first
	var img image.Image
	img, err = netpbm.Decode(file, nil)
	if err != nil {
		// If netpbm fails, try standard image decoders
		file.Seek(0, 0) // Reset file pointer to beginning
		img, _, err = image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %v", err)
		}
	}
	return img, nil
}

// ProcessScreenshotWithTrainingData processes a screenshot for OCR with optional training data
func ProcessScreenshotWithTrainingData(screenshotPath, trainingDataPath string, width, height int, debug bool) (*OCRResult, error) {
	img, err := decodeImageFile(screenshotPath)
	if err != nil {
		return nil, err
	}

	// Get image dimensions
	bounds := img.Bounds()
	imgWidth := bounds.Max.X - bounds.Min.X
	imgHeight := bounds.Max.Y - bounds.Min.Y

	// Calculate character cell dimensions
	cellWidth := imgWidth / width
	cellHeight := imgHeight / height

	if cellWidth <= 0 || cellHeight <= 0 {
		return nil, fmt.Errorf("invalid cell dimensions: %dx%d", cellWidth, cellHeight)
	}

	logging.Debug("Processing screenshot for OCR",
		"imageSize", fmt.Sprintf("%dx%d", imgWidth, imgHeight),
		"gridSize", fmt.Sprintf("%dx%d", width, height),
		"cellSize", fmt.Sprintf("%dx%d", cellWidth, cellHeight))

	// Extract character bitmaps
	charBitmaps := make([]CharacterBitmap, 0, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Calculate the pixel coordinates for the top-left corner of the character cell
			pixelX := x * cellWidth
			pixelY := y * cellHeight

			bitmap, err := extractCharacterBitmap(img, pixelX, pixelY, cellWidth, cellHeight)
			if err != nil {
				return nil, fmt.Errorf("failed to extract character bitmap at (%d,%d): %v", x, y, err)
			}
			charBitmaps = append(charBitmaps, bitmap)
		}
	}

	// Create OCR result
	result := &OCRResult{
		Width:       width,
		Height:      height,
		Text:        make([]string, height),
		CharBitmaps: charBitmaps,
	}

	// Try to load training data
	var trainingData *TrainingData
	if trainingDataPath != "" {
		trainingData, err = LoadTrainingData(trainingDataPath)
		if err == nil && trainingData != nil {
			// If training data exists, use it to recognize characters
			logging.Info("Using training data for character recognition", "path", trainingDataPath)
			if err := RecognizeCharacters(result, trainingData); err != nil {
				logging.Warn("Character recognition failed", "error", err)
			}
		} else {
			logging.Warn("No training data found, using basic recognition", "error", err)
		}
	} else {
		// Try default location as fallback
		defaultPath := filepath.Join(os.TempDir(), "qmp-ocr-training.json")
		trainingData, err = LoadTrainingData(defaultPath)
		if err == nil && trainingData != nil {
			// If training data exists, use it to recognize characters
			logging.Info("Using default training data for character recognition", "path", defaultPath)
			if err := RecognizeCharacters(result, trainingData); err != nil {
				logging.Warn("Character recognition failed", "error", err)
			}
		} else {
			logging.Warn("No training data found, using basic recognition", "error", err)
		}
	}

	return result, nil
}

// ProcessScreenshotWithCrop processes a screenshot for OCR with cropping
func ProcessScreenshotWithCrop(screenshotPath string, width, height int,
	startRow, endRow, startCol, endCol int, debug bool) (*OCRResult, error) {
	return ProcessScreenshotWithCropAndTrainingData(screenshotPath, "", width, height,
		startRow, endRow, startCol, endCol, debug)
}

// ProcessScreenshotWithCropAndTrainingData processes a screenshot for OCR with cropping and optional training data
func ProcessScreenshotWithCropAndTrainingData(screenshotPath, trainingDataPath string, width, height int,
	startRow, endRow, startCol, endCol int, debug bool) (*OCRResult, error) {
	img, err := decodeImageFile(screenshotPath)
	if err != nil {
		return nil, err
	}

	// Get image dimensions
	bounds := img.Bounds()
	imgWidth := bounds.Max.X - bounds.Min.X
	imgHeight := bounds.Max.Y - bounds.Min.Y

	// Calculate character cell dimensions
	cellWidth := imgWidth / width
	cellHeight := imgHeight / height

	if cellWidth <= 0 || cellHeight <= 0 {
		return nil, fmt.Errorf("invalid cell dimensions: %dx%d", cellWidth, cellHeight)
	}

	// Validate crop parameters
	if startRow < 0 || endRow >= height || startRow > endRow {
		return nil, fmt.Errorf("invalid row crop range: %d-%d (valid range: 0-%d)", startRow, endRow, height-1)
	}
	if startCol < 0 || endCol >= width || startCol > endCol {
		return nil, fmt.Errorf("invalid column crop range: %d-%d (valid range: 0-%d)", startCol, endCol, width-1)
	}

	// Calculate cropped dimensions
	croppedWidth := endCol - startCol + 1
	croppedHeight := endRow - startRow + 1

	logging.Debug("Processing cropped screenshot for OCR",
		"imageSize", fmt.Sprintf("%dx%d", imgWidth, imgHeight),
		"gridSize", fmt.Sprintf("%dx%d", width, height),
		"cellSize", fmt.Sprintf("%dx%d", cellWidth, cellHeight),
		"cropArea", fmt.Sprintf("rows %d-%d, cols %d-%d", startRow, endRow, startCol, endCol),
		"croppedSize", fmt.Sprintf("%dx%d", croppedWidth, croppedHeight))

	// Extract character bitmaps for the cropped area
	charBitmaps := make([]CharacterBitmap, 0, croppedWidth*croppedHeight)
	for y := startRow; y <= endRow; y++ {
		for x := startCol; x <= endCol; x++ {
			// Calculate the pixel coordinates for the top-left corner of the character cell
			pixelX := x * cellWidth
			pixelY := y * cellHeight

			bitmap, err := extractCharacterBitmap(img, pixelX, pixelY, cellWidth, cellHeight)
			if err != nil {
				return nil, fmt.Errorf("failed to extract character bitmap at (%d,%d): %v", x, y, err)
			}
			charBitmaps = append(charBitmaps, bitmap)
		}
	}

	// Create OCR result
	result := &OCRResult{
		Width:       croppedWidth,
		Height:      croppedHeight,
		Text:        make([]string, croppedHeight),
		CharBitmaps: charBitmaps,
	}

	// Try to load training data
	var trainingData *TrainingData
	if trainingDataPath != "" {
		trainingData, err = LoadTrainingData(trainingDataPath)
		if err == nil && trainingData != nil {
			// If training data exists, use it to recognize characters
			logging.Info("Using training data for character recognition", "path", trainingDataPath)
			if err := RecognizeCharacters(result, trainingData); err != nil {
				logging.Warn("Character recognition failed", "error", err)
			}
		} else {
			logging.Warn("No training data found, using basic recognition", "error", err)
		}
	} else {
		// Try default location as fallback
		defaultPath := filepath.Join(os.TempDir(), "qmp-ocr-training.json")
		trainingData, err = LoadTrainingData(defaultPath)
		if err == nil && trainingData != nil {
			// If training data exists, use it to recognize characters
			logging.Info("Using default training data for character recognition", "path", defaultPath)
			if err := RecognizeCharacters(result, trainingData); err != nil {
				logging.Warn("Character recognition failed", "error", err)
			}
		} else {
			logging.Warn("No training data found, using basic recognition", "error", err)
		}
	}

	return result, nil
}

// min returns the minimum of two uint32 values
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two uint32 values
func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

// colorToKey converts a color to a string key for counting
func colorToKey(c color.Color) string {
	r, g, b, _ := c.RGBA()
	// Use 8-bit values to reduce color variations
	return fmt.Sprintf("%d,%d,%d", r>>8, g>>8, b>>8)
}

// findMostCommonColor finds the most frequent color in a slice of colors
func findMostCommonColor(pixels []color.Color) color.Color {
	if len(pixels) == 0 {
		return color.RGBA{255, 255, 255, 255} // default to white
	}

	colorCounts := make(map[string]int)
	colorMap := make(map[string]color.Color)

	// Count occurrences of each color
	for _, c := range pixels {
		key := colorToKey(c)
		colorCounts[key]++
		colorMap[key] = c
	}

	// Find the most common color
	maxCount := 0
	var mostCommonKey string
	for key, count := range colorCounts {
		if count > maxCount {
			maxCount = count
			mostCommonKey = key
		}
	}

	return colorMap[mostCommonKey]
}

// isPixelDifferentFromBackground determines if a pixel color is significantly different from background
func isPixelDifferentFromBackground(r, g, b, bgR, bgG, bgB uint32) bool {
	// Convert to 8-bit for easier comparison
	r8, g8, b8 := r>>8, g>>8, b>>8
	bgR8, bgG8, bgB8 := bgR>>8, bgG>>8, bgB>>8

	// Calculate color distance (simple Euclidean distance)
	dr := int(r8) - int(bgR8)
	dg := int(g8) - int(bgG8)
	db := int(b8) - int(bgB8)

	distance := dr*dr + dg*dg + db*db

	// If the distance is above a threshold, consider it text
	// This threshold may need tuning - starting with a low value to catch colored text
	threshold := 30 * 30 // roughly 30 units difference in any channel

	return distance > threshold
}

// extractCharacterBitmap extracts a bitmap for a single character cell
func extractCharacterBitmap(img image.Image, x, y, cellWidth, cellHeight int) (CharacterBitmap, error) {
	// Create a new bitmap
	bitmap := CharacterBitmap{
		Width:  cellWidth,
		Height: cellHeight,
		Data:   make([][]bool, cellHeight),
		Colors: make([][]color.Color, cellHeight),
	}

	// Initialize the bitmap data
	for i := 0; i < cellHeight; i++ {
		bitmap.Data[i] = make([]bool, cellWidth)
		bitmap.Colors[i] = make([]color.Color, cellWidth)
	}

	// First pass: collect all pixel colors to determine background color
	var pixels []color.Color
	for cy := 0; cy < cellHeight; cy++ {
		for cx := 0; cx < cellWidth; cx++ {
			c := img.At(x+cx, y+cy)
			pixels = append(pixels, c)
		}
	}

	// Find the most common color (background color)
	bgColor := findMostCommonColor(pixels)
	bgR, bgG, bgB, _ := bgColor.RGBA()

	// Debug output for background color detection
	logging.Debug("Character bitmap extraction",
		"position", fmt.Sprintf("(%d,%d)", x, y),
		"cellSize", fmt.Sprintf("%dx%d", cellWidth, cellHeight),
		"backgroundRGB", fmt.Sprintf("(%d,%d,%d)", bgR>>8, bgG>>8, bgB>>8))

	// Second pass: extract the bitmap data using background color as reference
	for cy := 0; cy < cellHeight; cy++ {
		for cx := 0; cx < cellWidth; cx++ {
			// Get the pixel color
			c := img.At(x+cx, y+cy)
			r, g, b, _ := c.RGBA()

			// Store the original color
			bitmap.Colors[cy][cx] = c

			// Check if this pixel is significantly different from background
			isText := isPixelDifferentFromBackground(r, g, b, bgR, bgG, bgB)
			bitmap.Data[cy][cx] = isText
		}
	}

	return bitmap, nil
}

// SaveOCRResult saves OCR results to a file
func SaveOCRResult(result *OCRResult, outputPath string, debug bool) error {
	var output string

	if debug {
		// In debug mode, include ASCII representation and JSON data
		output = formatDebugOutput(result)
	} else {
		// In normal mode, just include the recognized text
		output = formatTextOutput(result)
	}

	return os.WriteFile(outputPath, []byte(output), 0644)
}

// formatTextOutput formats OCR results as plain text
func formatTextOutput(result *OCRResult) string {
	var sb strings.Builder

	for _, line := range result.Text {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatDebugOutput formats OCR results with debug information
func formatDebugOutput(result *OCRResult) string {
	var sb strings.Builder

	// Add text representation
	sb.WriteString("OCR Text Output:\n")
	sb.WriteString("---------------\n")
	for _, line := range result.Text {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Add ASCII representation of bitmaps
	sb.WriteString("Character Bitmaps:\n")
	sb.WriteString("----------------\n")

	idx := 0
	for y := 0; y < result.Height; y++ {
		for x := 0; x < result.Width; x++ {
			if x > 0 {
				sb.WriteString(" ")
			}

			bitmap := result.CharBitmaps[idx]
			sb.WriteString(formatBitmapASCII(bitmap))
			idx++
		}
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// formatBitmapASCII formats a bitmap as ASCII art
func formatBitmapASCII(bitmap CharacterBitmap) string {
	var sb strings.Builder

	for y := 0; y < bitmap.Height; y++ {
		for x := 0; x < bitmap.Width; x++ {
			if bitmap.Data[y][x] {
				sb.WriteString("#")
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SaveTrainingData saves OCR training data to a file, sorted by character value
func SaveTrainingData(data *TrainingData, outputPath string) error {
	// Create a sorted representation of the training data
	sortedData := createSortedTrainingData(data)

	jsonData, err := json.MarshalIndent(sortedData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal training data: %v", err)
	}

	return os.WriteFile(outputPath, jsonData, 0644)
}

// createSortedTrainingData creates a sorted representation of training data, sorted by character value
func createSortedTrainingData(data *TrainingData) map[string]interface{} {
	// Create a slice of key-value pairs for sorting
	type bitmapEntry struct {
		HexBitmap string `json:"hexBitmap"`
		Char      string `json:"char"`
	}

	var entries []bitmapEntry
	for hexBitmap, char := range data.BitmapMap {
		entries = append(entries, bitmapEntry{
			HexBitmap: hexBitmap,
			Char:      char,
		})
	}

	// Sort by character value
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Char < entries[j].Char
	})

	// Rebuild the map in sorted order
	sortedMap := make(map[string]string)
	for _, entry := range entries {
		sortedMap[entry.HexBitmap] = entry.Char
	}

	return map[string]interface{}{
		"bitmapMap": sortedMap,
	}
}

// LoadTrainingData loads OCR training data from a file
func LoadTrainingData(inputPath string) (*TrainingData, error) {
	logging.Debug("Loading training data", "path", inputPath)

	jsonData, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read training data: %v", err)
	}

	var data TrainingData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal training data: %v", err)
	}

	// Ensure BitmapMap is initialized
	if data.BitmapMap == nil {
		data.BitmapMap = make(map[string]string)
	}

	logging.Debug("Training data loaded successfully",
		"path", inputPath,
		"characterCount", len(data.BitmapMap))

	// Show first few entries for debugging
	count := 0
	for hex, char := range data.BitmapMap {
		if count < 5 {
			logging.Debug("Training data entry",
				"hex", func() string {
					if len(hex) > 20 { return hex[:20] + "..." }
					return hex
				}(),
				"char", char)
		}
		count++
		if count >= 5 {
			break
		}
	}

	return &data, nil
}

// SaveBitmapAsPNG saves a character bitmap as a PNG image
func SaveBitmapAsPNG(bitmap CharacterBitmap, outputPath string) error {
	// Create a new image
	img := image.NewRGBA(image.Rect(0, 0, bitmap.Width, bitmap.Height))

	// Fill the image based on bitmap data
	for y := 0; y < bitmap.Height; y++ {
		for x := 0; x < bitmap.Width; x++ {
			if bitmap.Data[y][x] {
				img.Set(x, y, bitmap.Colors[y][x])
			} else {
				img.Set(x, y, color.White)
			}
		}
	}

	// Create output file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Encode and save the image
	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to encode PNG: %v", err)
	}

	return nil
}

// ExtractTrainingData extracts training data from a screenshot with known characters
func ExtractTrainingData(screenshotPath string, width, height int, knownChars string) (*TrainingData, error) {
	// Process the screenshot
	result, err := ProcessScreenshot(screenshotPath, width, height, true)
	if err != nil {
		return nil, err
	}

	// Create training data
	data := &TrainingData{
		BitmapMap: make(map[string]string),
	}

	// Map characters to bitmaps
	// This is a simplified approach assuming characters are in order from top-left
	charIndex := 0
	for i, bitmap := range result.CharBitmaps {
		if charIndex >= len(knownChars) {
			break
		}

		// Skip empty bitmaps (all white or all black)
		if isEmptyBitmap(bitmap) {
			continue
		}

		char := string(knownChars[charIndex])
		bitmap.Char = char

		// Store in BitmapMap (hex string to character)
		hexBitmap := FormatBitmapAsHex(&bitmap)
		data.BitmapMap[hexBitmap] = char

		charIndex++

		logging.Debug("Mapped character to bitmap", "char", char, "position", i)
	}

	if charIndex == 0 {
		return nil, fmt.Errorf("no characters were mapped to bitmaps")
	}

	return data, nil
}

// isEmptyBitmap checks if a bitmap is empty (all white or all black)
func isEmptyBitmap(bitmap CharacterBitmap) bool {
	allTrue := true
	allFalse := true

	for y := 0; y < bitmap.Height; y++ {
		for x := 0; x < bitmap.Width; x++ {
			if bitmap.Data[y][x] {
				allFalse = false
			} else {
				allTrue = false
			}

			if !allTrue && !allFalse {
				return false
			}
		}
	}

	return true
}

// RecognizeCharacters recognizes characters in the OCR result using training data
func RecognizeCharacters(result *OCRResult, trainingData *TrainingData) error {
	if trainingData == nil || len(trainingData.BitmapMap) == 0 {
		return fmt.Errorf("no training data available")
	}

	// Process each character bitmap
	idx := 0
	for y := 0; y < result.Height; y++ {
		var lineBuilder strings.Builder

		for x := 0; x < result.Width; x++ {
			if idx >= len(result.CharBitmaps) {
				break
			}

			bitmap := result.CharBitmaps[idx]
			recognizedChar := recognizeCharacter(bitmap, trainingData)
			lineBuilder.WriteString(recognizedChar)

			// Update the bitmap's Char field so training system knows it's recognized
			result.CharBitmaps[idx].Char = recognizedChar
			idx++
		}

		result.Text[y] = lineBuilder.String()
	}

	return nil
}

// recognizeCharacter recognizes a single character using training data
func recognizeCharacter(bitmap CharacterBitmap, trainingData *TrainingData) string {
	// First try direct hex bitmap lookup (exact match)
	hexBitmap := FormatBitmapAsHex(&bitmap)

	// Debug logging to trace recognition
	logging.Debug("Recognizing character",
		"hexBitmap", hexBitmap,
		"trainingDataSize", len(trainingData.BitmapMap))

	if char, found := trainingData.BitmapMap[hexBitmap]; found {
		logging.Debug("Character recognized from training data",
			"hexBitmap", hexBitmap,
			"char", char)
		return char
	}

	// Debug: Log first few training data entries for comparison
	if len(trainingData.BitmapMap) > 0 {
		count := 0
		for trainedHex := range trainingData.BitmapMap {
			if count < 3 { // Show first 3 for comparison
				logging.Debug("Training data sample",
					"trainedHex", trainedHex,
					"matches", trainedHex == hexBitmap)
			}
			count++
			if count >= 3 {
				break
			}
		}
	}

	// No match found
	logging.Debug("Character not recognized", "hexBitmap", hexBitmap)
	return "?"
}

// compareBitmaps compares two bitmaps and returns a similarity score
// Lower score means more similar
func compareBitmaps(a, b CharacterBitmap) float64 {
	// Simple implementation: count the number of differing pixels
	// and normalize by the total number of pixels

	// First, we need to resize the bitmaps to the same dimensions
	// For simplicity, we'll use the dimensions of bitmap a
	resizedB := resizeBitmap(b, a.Width, a.Height)

	diffCount := 0
	totalPixels := a.Width * a.Height

	for y := 0; y < a.Height; y++ {
		for x := 0; x < a.Width; x++ {
			if a.Data[y][x] != resizedB.Data[y][x] {
				diffCount++
			}
		}
	}

	return float64(diffCount) / float64(totalPixels)
}

// resizeBitmap resizes a bitmap to the specified dimensions
func resizeBitmap(bitmap CharacterBitmap, width, height int) CharacterBitmap {
	if bitmap.Width == width && bitmap.Height == height {
		return bitmap
	}

	result := CharacterBitmap{
		Width:  width,
		Height: height,
		Data:   make([][]bool, height),
		Colors: make([][]color.Color, height),
		Char:   bitmap.Char,
	}

	// Initialize the result data
	for i := range result.Data {
		result.Data[i] = make([]bool, width)
		result.Colors[i] = make([]color.Color, width)
	}

	// Simple nearest neighbor scaling
	for y := 0; y < height; y++ {
		srcY := y * bitmap.Height / height
		if srcY >= bitmap.Height {
			srcY = bitmap.Height - 1
		}

		for x := 0; x < width; x++ {
			srcX := x * bitmap.Width / width
			if srcX >= bitmap.Width {
				srcX = bitmap.Width - 1
			}

			result.Data[y][x] = bitmap.Data[srcY][srcX]
			result.Colors[y][x] = bitmap.Colors[srcY][srcX]
		}
	}

	return result
}

// ExtractSingleCharacter extracts a single character bitmap from a screenshot
func ExtractSingleCharacter(screenshotPath string, width, height int, row, col int) (*CharacterBitmap, error) {
	img, err := decodeImageFile(screenshotPath)
	if err != nil {
		return nil, err
	}

	// Get image dimensions
	bounds := img.Bounds()
	imgWidth := bounds.Max.X - bounds.Min.X
	imgHeight := bounds.Max.Y - bounds.Min.Y

	// Calculate character cell dimensions
	cellWidth := imgWidth / width
	cellHeight := imgHeight / height

	if cellWidth <= 0 || cellHeight <= 0 {
		return nil, fmt.Errorf("invalid cell dimensions: %dx%d", cellWidth, cellHeight)
	}

	// Validate row and column
	if row < 0 || row >= height {
		return nil, fmt.Errorf("invalid row: %d (valid range: 0-%d)", row, height-1)
	}
	if col < 0 || col >= width {
		return nil, fmt.Errorf("invalid column: %d (valid range: 0-%d)", col, width-1)
	}

	// Calculate the pixel coordinates for the top-left corner of the character cell
	x := col * cellWidth
	y := row * cellHeight

	// Extract the character bitmap
	bitmap, err := extractCharacterBitmap(img, x, y, cellWidth, cellHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to extract character bitmap at (%d,%d): %v", row, col, err)
	}

	return &bitmap, nil
}

// FormatBitmapAsHex formats a bitmap as a hex string
// This is an exported version that can be used by other packages
func FormatBitmapAsHex(bitmap *CharacterBitmap) string {
	var hexStr strings.Builder
	hexStr.WriteString("0x")

	for y := 0; y < bitmap.Height; y++ {
		var rowValue uint32

		// Convert row to binary
		for x := 0; x < bitmap.Width; x++ {
			if y < len(bitmap.Data) && x < len(bitmap.Data[y]) && bitmap.Data[y][x] {
				// Set bit if pixel is on
				rowValue |= 1 << (bitmap.Width - 1 - x)
			}
		}

		// Format as hex without 0x prefix
		hexStr.WriteString(fmt.Sprintf("%0*X", (bitmap.Width+3)/4, rowValue))
	}

	return hexStr.String()
}

// CreateSimplifiedTrainingData creates a new training data object with only BitmapMap
// This helps with migrating existing training data to a more compact format
func CreateSimplifiedTrainingData(data *TrainingData) *TrainingData {
	// Create a new training data object with only BitmapMap
	simplified := &TrainingData{
		BitmapMap: make(map[string]string),
	}

	// Copy all BitmapMap entries
	for hexBitmap, char := range data.BitmapMap {
		simplified.BitmapMap[hexBitmap] = char
	}

	logging.Debug("Created simplified training data", "entries", len(simplified.BitmapMap))
	return simplified
}
