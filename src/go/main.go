package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Release struct {
	TagName string `json:"tag_name"`
}

func main() {
	if len(os.Args) < 5 {
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
		log.Fatalf("Failed to inspect Docker image: %v", err)
	}

	sizeBytes, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		log.Fatalf("Failed to parse image size: %v", err)
	}

	sizeGB := sizeBytes / (1024 * 1024 * 1024)
	fmt.Printf("Image size: \033[1;33m%.2f GB\033[0m\n", sizeGB)

	if sizeGB > 1 {
		if !allowLargeImage {
			fmt.Println("\033[1;31mError: The image size exceeds 1 GB.\033[0m")
			fmt.Println("\n\n#\tPass 'allow_large_image=true' to proceed.")
			if continueOnFail {
				fmt.Println("#\tPass 'continue_on_fail=false' to fail actions that don't pass the test.")
				fmt.Println("\033[1;33mCONTINUE POLICY ENABLED...\033[0m")
				return false
			}
			os.Exit(1)
		} else {
			fmt.Println("\033[1;32mLarge image allowed. Continuing...\033[0m")
		}
	}
	return true
}
func removeANSICodes(input string) string {
	ansiRegex := regexp.MustCompile("\033\\[[0-9;]*m")
	return ansiRegex.ReplaceAllString(input, "")
}
func checkImage(imageName, ciConfig string, continueOnFail bool, reportEnabled bool) {
	var multiWriter io.Writer = os.Stdout
	var reportFile *os.File
	if reportEnabled {
		var err error
		reportFile, err = os.Create("/tmp/DIVE_REPORT.md")
		if err != nil {
			log.Fatalf("Failed to create report file: %v", err)
		}
		defer reportFile.Close()
		multiWriter = io.MultiWriter(os.Stdout, reportFile)
	}

	cmd := exec.Command("dive", "--ci-config", ciConfig, imageName)
	cmd.Env = append(os.Environ(), "CI=true")
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to create stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("Failed to create stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start command: %v", err)
	}

	go func() {
		io.Copy(multiWriter, stdoutPipe)
	}()
	go func() {
		io.Copy(multiWriter, stderrPipe)
	}()
	if err := cmd.Wait(); err != nil {
		if continueOnFail {
			fmt.Println("\033[1;33mCONTINUE POLICY ENABLED...\033[0m")
			fmt.Println("\n\n#\tPass 'continue_on_fail=false' to fail actions that don't pass the test.")
		} else {
			log.Fatalf("Dive analysis failed: %v", err)
		}
	}
	if reportEnabled {
		content, err := os.ReadFile("/tmp/DIVE_REPORT.md")
		if err != nil {
			log.Fatalf("Failed to read report file: %v", err)
		}
		cleanedContent := removeANSICodes(string(content))
		if err := os.WriteFile("/tmp/DIVE_REPORT.md", []byte(cleanedContent), 0644); err != nil {
			log.Fatalf("Failed to write cleaned report file: %v", err)
		}
	}
}
