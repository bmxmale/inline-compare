package main

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

var debug bool

func main() {
	lineLimit := flag.Int("lines", 50, "Number of lines to compare for large files")
	sizeLimit := flag.Int("size", 100, "File size limit in MB for comparing last lines")
	useCache := flag.Bool("use-cache", false, "Use existing checksum CSV files instead of regenerating new ones")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.Parse()

	if len(flag.Args()) != 2 {
		fmt.Println("Usage: compare [options] <dir1> <dir2>")
		return
	}

	dir1 := filepath.Clean(flag.Arg(0))
	dir2 := filepath.Clean(flag.Arg(1))
	outputDir := filepath.Clean(dir1 + "-" + dir2)

	// Create the output directory
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	fmt.Printf("# Compare %s and %s\n", dir1, dir2)

	checksums1, err := generateChecksums(dir1, *useCache, outputDir)
	if err != nil {
		fmt.Printf("Error generating checksums for %s: %v\n", dir1, err)
		return
	}

	checksums2, err := generateChecksums(dir2, *useCache, outputDir)
	if err != nil {
		fmt.Printf("Error generating checksums for %s: %v\n", dir2, err)
		return
	}

	err = generateCombinedCSV(checksums1, checksums2, dir1, dir2, outputDir)
	if err != nil {
		fmt.Printf("Error generating combined CSV: %v\n", err)
		return
	}
	fmt.Printf("# Combined CSV generated at %s\n", filepath.Join(outputDir, "diff.csv"))

	diffCount, err := compareFilesInCSV(dir1, dir2, *sizeLimit, *lineLimit, outputDir)
	if err != nil {
		fmt.Printf("Error comparing files: %v\n", err)
		return
	}

	fmt.Printf("# Total differences found: %d (%s)\n", diffCount, filepath.Join(outputDir, "diffs"))
}

func generateChecksums(dir string, useCache bool, outputDir string) (map[string]string, error) {
	checksums := make(map[string]string)
	csvFile := filepath.Join(outputDir, filepath.Base(dir)+"-checksums.csv")

	if useCache {
		file, err := os.Open(csvFile)
		if err == nil {
			defer file.Close()
			reader := csv.NewReader(file)
			records, err := reader.ReadAll()
			if err == nil {
				for _, record := range records {
					checksums[record[0]] = record[1]
				}
				return checksums, nil
			}
		}
	} else {
		// Delete existing checksum file if it exists
		if err := os.Remove(csvFile); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(dir, file.Name())
			checksum, err := fileChecksum(filePath)
			if err != nil {
				return nil, err
			}
			checksums[file.Name()] = checksum
			err = updateCSV(csvFile, file.Name(), checksum)
			if err != nil {
				return nil, err
			}
			// Print the file checksum
			fmt.Printf(" - %s: %s\n", filePath, checksum)
		}
	}

	fmt.Printf("# Checksums for %s generated (%s)\n", dir, csvFile)

	return checksums, nil
}

func fileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func updateCSV(csvFile, fileName, checksum string) error {
	file, err := os.OpenFile(csvFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{fileName, checksum})
	if err != nil {
		return err
	}

	return nil
}

func generateCombinedCSV(checksums1, checksums2 map[string]string, dir1, dir2, outputDir string) error {
	outputFile, err := os.Create(filepath.Join(outputDir, "diff.csv"))
	if err != nil {
		return err
	}
	defer outputFile.Close()

	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	// Write headers
	err = writer.Write([]string{"File Name", "Checksum " + dir1, "Checksum " + dir2})
	if err != nil {
		return err
	}

	// Get all unique file names
	fileNames := make(map[string]bool)
	for fileName := range checksums1 {
		fileNames[fileName] = true
	}
	for fileName := range checksums2 {
		fileNames[fileName] = true
	}

	// Sort file names
	var sortedFileNames []string
	for fileName := range fileNames {
		sortedFileNames = append(sortedFileNames, fileName)
	}
	sort.Strings(sortedFileNames)

	// Write data
	for _, fileName := range sortedFileNames {
		checksum1 := checksums1[fileName]
		checksum2 := checksums2[fileName]
		if checksum1 != checksum2 {
			err = writer.Write([]string{fileName, checksum1, checksum2})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func compareFilesInCSV(dir1, dir2 string, sizeLimit int, lineLimit int, outputDir string) (int, error) {
	file, err := os.Open(filepath.Join(outputDir, "diff.csv"))
	if err != nil {
		return 0, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return 0, err
	}

	diffDir := filepath.Join(outputDir, "diffs")
	err = os.MkdirAll(diffDir, 0755)
	if err != nil {
		return 0, err
	}

	diffCount := 0
	fmt.Printf("# Start comparing files\n")
	for _, record := range records[1:] { // Skip header
		file1 := filepath.Join(dir1, record[0])
		file2 := filepath.Join(dir2, record[0])

		_, err1 := os.Stat(file1)
		_, err2 := os.Stat(file2)

		if os.IsNotExist(err1) {
			// file1 does not exist, copy file2 to diffs directory
			err = copyFile(file2, filepath.Join(diffDir, record[0]))
			if err != nil {
				return 0, err
			}
			diffCount++
		} else if os.IsNotExist(err2) {
			// file2 does not exist, copy file1 to diffs directory
			err = copyFile(file1, filepath.Join(diffDir, record[0]))
			if err != nil {
				return 0, err
			}
			diffCount++
		} else {
			// Both files exist, compare them
			sizeLimitInBytes := sizeLimit * 1024 * 1024
			diffFile := filepath.Join(diffDir, record[0]+".diff")
			err = generateDiff(file1, file2, diffFile, sizeLimitInBytes, lineLimit)
			if err != nil {
				return 0, err
			}
			diffCount++
		}
	}

	fmt.Printf("# Files compared and differences stored in %s\n", diffDir)

	return diffCount, nil
}

func generateDiff(file1, file2, diffFile string, sizeLimit, lineLimit int) error {
	// Check if the diff file already exists and remove it
	if _, err := os.Stat(diffFile); err == nil {
		if err := os.Remove(diffFile); err != nil {
			return fmt.Errorf("failed to remove existing diff file: %v", err)
		}
	}

	info1, err := os.Stat(file1)
	if err != nil {
		return err
	}
	info2, err := os.Stat(file2)
	if err != nil {
		return err
	}

	var content1, content2 []byte

	if debug {
		fmt.Printf("// Size limit: %d\n", sizeLimit)
		fmt.Printf("// File Size 1: %d\n", info1.Size())
		fmt.Printf("// File Size 2: %d\n", info2.Size())
	}

	if info1.Size() > int64(sizeLimit) || info2.Size() > int64(sizeLimit) {
		if debug {
			fmt.Printf("// large files detected: %s - %s, comparing last %d lines\n", file1, file2, lineLimit)
		}
		content1, err = readLastLines(file1, lineLimit)
		if err != nil {
			return err
		}
		content2, err = readLastLines(file2, lineLimit)
		if err != nil {
			return err
		}
	} else {
		if debug {
			fmt.Printf("// comparing entire files %s - %s \n", file1, file2)
		}
		content1, err = readFileInChunks(file1)
		if err != nil {
			return err
		}
		content2, err = readFileInChunks(file2)
		if err != nil {
			return err
		}
	}

	tmpFile1, err := os.CreateTemp("", "file1-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile1.Name())

	tmpFile2, err := os.CreateTemp("", "file2-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile2.Name())

	if _, err := tmpFile1.Write(content1); err != nil {
		return err
	}
	if _, err := tmpFile2.Write(content2); err != nil {
		return err
	}

	cmd := exec.Command("diff", "-u", tmpFile1.Name(), tmpFile2.Name())
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return err
	}

	err = os.WriteFile(diffFile, output, 0644)
	if err != nil {
		return err
	}

	fmt.Printf(" - diff generated for %s (%s) and %s (%s)\n", file1, humanReadableSize(info1.Size()), file2, humanReadableSize(info2.Size()))
	if debug {
		fmt.Printf(" __________________________________________________________\n")
	}

	return nil
}

func readLastLines(filePath string, n int) ([]byte, error) {
	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", n), filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(filepath.Clean(dst))
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	fmt.Printf("# File copied from %s to %s\n", src, dst)

	return nil
}

func humanReadableSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func readFileInChunks(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var content []byte
	buf := make([]byte, 1024*1024) // 1 MB buffer
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}
		content = append(content, buf[:n]...)
	}

	return content, nil
}
