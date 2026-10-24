package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	reportPath = "/tmp/DIVE_REPORT.md"
	maxSizeGB  = 1.0
)

var ansiRegex = regexp.MustCompile("\033\\[[0-9;]*m")

func main() {
	log.SetFlags(0)

	// program name + 5 arguments
	if len(os.Args) < 6 {
		log.Fatalf("Usage: %s <image_name> <ci_config> <allow_large_image> <continue_on_fail> <report>", os.Args[0])
	}

	imageName := os.Args[1]
	ciConfig := os.Args[2]
	allowLargeImage := os.Args[3] == "true"
	continueOnFail := os.Args[4] == "true"
	reportEnabled := os.Args[5] == "true"

	fmt.Println("Checking Docker image size...")
	if !checkImageSize(imageName, allowLargeImage, continueOnFail) {
		return
	}

	fmt.Printf("Running Dive analysis on image: %s with CI config: %s\n", imageName, ciConfig)
	checkImage(imageName, ciConfig, continueOnFail, reportEnabled)
}

func checkImageSize(imageName string, allowLargeImage, continueOnFail bool) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName, "--format={{.Size}}")
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to inspect Docker image %q (is it built/pulled locally?): %v", imageName, err)
	}

	sizeBytes, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		log.Fatalf("Failed to parse image size: %v", err)
	}

	sizeGB := sizeBytes / (1024 * 1024 * 1024)
	fmt.Printf("Image size: \033[1;33m%.2f GB\033[0m\n", sizeGB)

	if sizeGB <= maxSizeGB {
		return true
	}

	if allowLargeImage {
		fmt.Println("\033[1;32mLarge image allowed. Continuing...\033[0m")
		return true
	}

	fmt.Println("\033[1;31mError: The image size exceeds 1 GB.\033[0m")
	fmt.Println("\n\n#\tPass 'allow_large_image=true' to proceed.")
	if continueOnFail {
		fmt.Println("#\tPass 'continue_on_fail=false' to fail actions that don't pass the test.")
		fmt.Println("\033[1;33mCONTINUE POLICY ENABLED...\033[0m")
		fmt.Println("::warning title=Dive analysis::Image is larger than 1 GB, analysis skipped (set allow_large_image=true)")
		return false
	}
	fmt.Println("::error title=Dive analysis::Image is larger than 1 GB, analysis skipped (set allow_large_image=true)")
	os.Exit(1)
	return false
}

func checkImage(imageName, ciConfig string, continueOnFail, reportEnabled bool) {
	if _, err := os.Stat(ciConfig); err != nil {
		log.Fatalf("Dive CI config not found: %v", err)
	}

	var out io.Writer = os.Stdout
	var reportFile *os.File
	if reportEnabled {
		f, err := os.Create(reportPath)
		if err != nil {
			log.Fatalf("Failed to create report file: %v", err)
		}
		reportFile = f
		out = io.MultiWriter(os.Stdout, f)
	}

	fmt.Println("Analyzing image...")

	cmd := exec.Command("dive", "--ci-config", ciConfig, imageName)
	cmd.Env = append(os.Environ(), "CI=true")
	// The same writer for both streams: exec serializes the writes
	// and Run waits until all output has been copied.
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()

	if reportFile != nil {
		reportFile.Close()
		if cleanErr := cleanReport(); cleanErr != nil {
			fmt.Printf("::warning title=Dive analysis::Failed to clean report: %v\n", cleanErr)
		}
	}

	if err == nil {
		return
	}

	// dive could not be started at all: this is an environment problem,
	// not a failed quality gate, so continue_on_fail must not hide it.
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		log.Fatalf("Failed to run dive: %v", err)
	}

	if continueOnFail {
		fmt.Println("\033[1;33mCONTINUE POLICY ENABLED...\033[0m")
		fmt.Println("\n\n#\tPass 'continue_on_fail=false' to fail actions that don't pass the test.")
		fmt.Println("::warning title=Dive analysis::Image did not pass the checks (continue_on_fail=true)")
		return
	}

	fmt.Println("::error title=Dive analysis::Image did not pass the checks")
	code := exitErr.ExitCode()
	if code <= 0 {
		code = 1
	}
	os.Exit(code)
}

func cleanReport() error {
	content, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	cleaned := ansiRegex.ReplaceAllString(string(content), "")
	return os.WriteFile(reportPath, []byte(cleaned), 0644)
}
