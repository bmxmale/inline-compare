
# Inline Compare

The `inline-compare` script is a Go program designed to compare files in two directories by generating and comparing checksums. It allows users to specify various options such as line limits for large files, file size limits, and whether to use cached checksum files.

Script was created to compare 2 dumps of the same database, it outputs differences in a new directory.

## Features

- Generate checksums for files in two directories.
- Compare files based on their checksums.
- Generate a CSV file with differences.
- Compare large files by their last lines if they exceed a specified size limit.
- Option to use existing checksum CSV files to speed up the comparison process.

## Usage

1. **Clone the repository:**

   ```sh
   git clone git@github.com:bmxmale/inline-compare.git
   cd inline-compare
   ```

2. **Build the script:**

   ```sh
   # Build for Linux
   GOOS=linux GOARCH=amd64 go build -o bin/inline-compare inline-compare.go
   
   # Build for macOS with Apple M3 processor
   GOOS=darwin GOARCH=arm64 go build -o bin/inline-compare inline-compare.go
   ```

3. **Run the script:**

   ```sh
   ./inline-compare [options] <dir1> <dir2>
   ```

4. **Options:**

    - `-lines`: Number of lines to compare for large files (default: 50).
    - `-size`: File size limit in MB for comparing last lines (default: 100).
    - `-use-cache`: Use existing checksum CSV files instead of regenerating new ones.
    - `-debug`: Enable debug mode to display additional information.

## Example

<p align="center" >
<img src="docs/inline-compare.svg" />
</p> 

## Configuration Options

- **Lines:** Number of lines to compare for large files.
- **Size:** File size limit in MB for comparing last lines.
- **Use Cache:** Use existing checksum CSV files to speed up the comparison process.
- **Debug:** Enable debug mode to display additional information during execution.

## Acknowledgements

This software was created with the strong support of GitHub Copilot ❤️, an AI-powered code completion tool that helps developers write code faster and with greater accuracy.

With ❤️ from Poland 🇵🇱.
