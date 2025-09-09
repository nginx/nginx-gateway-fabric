package framework

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// CreateResultsDir creates and returns the name of the results directory for a test.
func CreateResultsDir(testName, version string) (string, error) {
	pwd, err := GetWorkingDir()
	if err != nil {
		return "", err
	}

	dirName := filepath.Join(filepath.Dir(pwd), "results", testName, version)

	return dirName, MkdirAll(dirName, 0o777)
}

// CreateResultsFile creates and returns the results file for a test.
func CreateResultsFile(filename string) (*os.File, error) {
	GinkgoWriter.Printf("Creating results file %q\n", filename)
	outFile, err := OpenFile(filename, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during creating results file %q, error: %s\n", filename, err)

		return nil, err
	}

	return outFile, nil
}

// CreateResultsFilename returns the name of the results file.
func CreateResultsFilename(ext, base string, plusEnabled bool) string {
	name := fmt.Sprintf("%s-oss.%s", base, ext)
	if plusEnabled {
		name = fmt.Sprintf("%s-plus.%s", base, ext)
	}

	return name
}

// WriteSystemInfoToFile writes the cluster system info to the given file.
func WriteSystemInfoToFile(file *os.File, ci ClusterInfo, plus bool) error {
	clusterType := "Local"
	if ci.IsGKE {
		clusterType = "GKE"
	}

	commit, date, dirty := GetBuildInfo()

	//nolint:lll
	text := fmt.Sprintf(
		"# Results\n\n## Test environment\n\nNGINX Plus: %v\n\nNGINX Gateway Fabric:\n\n- Commit: %s\n- Date: %s\n- Dirty: %v\n\n%s Cluster:\n\n- Node count: %d\n- k8s version: %s\n- vCPUs per node: %d\n- RAM per node: %s\n- Max pods per node: %d\n",
		plus, commit, date, dirty, clusterType, ci.NodeCount, ci.K8sVersion, ci.CPUCountPerNode, ci.MemoryPerNode, ci.MaxPodsPerNode,
	)
	if _, err := fmt.Fprint(file, text); err != nil {
		GinkgoWriter.Printf("ERROR occurred during writing system info to results file, error: %s\n", err)

		return err
	}
	if ci.IsGKE {
		if _, err := fmt.Fprintf(file, "- Zone: %s\n- Instance Type: %s\n", ci.GkeZone, ci.GkeInstanceType); err != nil {
			GinkgoWriter.Printf("ERROR occurred during writing GKE info to results file, error: %s\n", err)

			return err
		}
	}
	GinkgoWriter.Printf("Wrote system info to results file\n")

	return nil
}

func generatePNG(resultsDir, inputFilename, outputFilename, configFilename string) error {
	pwd, err := GetWorkingDir()
	if err != nil {
		return err
	}

	gnuplotCfg := filepath.Join(filepath.Dir(pwd), "scripts", configFilename)

	files := fmt.Sprintf("inputfile='%s';outputfile='%s'", inputFilename, outputFilename)
	cmd := exec.CommandContext(context.Background(), "gnuplot", "-e", files, "-c", gnuplotCfg)
	cmd.Dir = resultsDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		GinkgoWriter.Printf(
			"ERROR occurred during generating PNG %q using gnuplot, error: %s, output: %s\n",
			outputFilename,
			err,
			string(output),
		)

		return fmt.Errorf("failed to generate PNG: %w; output: %s", err, string(output))
	}

	return nil
}

// GenerateRequestsPNG generates a Requests PNG using gnuplot.
func GenerateRequestsPNG(resultsDir, inputFilename, outputFilename string) error {
	return generatePNG(resultsDir, inputFilename, outputFilename, "requests-plot.gp")
}

// GenerateTTRPNG generates a TTR PNG using gnuplot.
func GenerateTTRPNG(resultsDir, inputFilename, outputFilename string) error {
	return generatePNG(resultsDir, inputFilename, outputFilename, "ttr-plot.gp")
}

// GenerateCPUPNG generates a CPU usage PNG using gnuplot.
func GenerateCPUPNG(resultsDir, inputFilename, outputFilename string) error {
	return generatePNG(resultsDir, inputFilename, outputFilename, "cpu-plot.gp")
}

// GenerateMemoryPNG generates a Memory usage PNG using gnuplot.
func GenerateMemoryPNG(resultsDir, inputFilename, outputFilename string) error {
	return generatePNG(resultsDir, inputFilename, outputFilename, "memory-plot.gp")
}

// WriteMetricsResults writes the metrics results to the results file in text format.
func WriteMetricsResults(resultsFile *os.File, metrics *Metrics) error {
	reporter := vegeta.NewTextReporter(&metrics.Metrics)
	reporterErr := reporter.Report(resultsFile)
	if reporterErr != nil {
		GinkgoWriter.Printf("ERROR occurred during writing metrics results to results file, error: %s\n", reporterErr)
	}
	GinkgoWriter.Printf("Wrote metrics results to results file %q\n", resultsFile.Name())

	return reporterErr
}

// WriteContent writes basic content to the results file.
func WriteContent(resultsFile *os.File, content string) error {
	if _, err := fmt.Fprintln(resultsFile, content); err != nil {
		GinkgoWriter.Printf("ERROR occurred during writing content to results file, error: %s\n", err)

		return err
	}

	return nil
}

// GetWorkingDir returns the current working directory.
func GetWorkingDir() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during getting current working directory %q, error: %s\n", pwd, err)
	}

	return pwd, err
}

// CreateFile creates a new file with the given name.
func CreateFile(fileName string) (*os.File, error) {
	file, err := os.Create(fileName)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during creating file %q, error: %s\n", fileName, err)
	}

	return file, err
}

// OpenFile opens an existing file with the given name.
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during openning results file %q, error: %s\n", name, err)
	}

	return file, err
}

// MkdirAll creates a directory with the specified permissions.
func MkdirAll(path string, perm os.FileMode) error {
	err := os.MkdirAll(path, perm)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during creating directory %q, error: %s\n", path, err)
	}

	return err
}

// ReadFile reads the contents of a file.
func ReadFile(file string) ([]byte, error) {
	result, err := os.ReadFile(file)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during reading file %q, error: %s\n", file, err)
	}

	return result, err
}

// WriteString writes a string to the given file.
func WriteString(file *os.File, content string) (int, error) {
	result, err := io.WriteString(file, content)
	if err != nil {
		GinkgoWriter.Printf("ERROR writing error log file: %v\n", err)
	}
	return result, err
}

// WriteCSVRecord writes a CSV record using the given writer.
func WriteCSVRecord(writer *csv.Writer, record []string) error {
	err := writer.Write(record)
	if err != nil {
		GinkgoWriter.Printf("ERROR writing CSV record: %v\n", err)
	}

	return err
}

// UserHomeDir returns the user's home directory.
func UserHomeDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		GinkgoWriter.Printf("ERROR getting user home directory, error: %s\n", err)
	}
	GinkgoWriter.Printf("User home directory is %q\n", dir)

	return dir, err
}

// Remove removes the specified file or empty directory.
func Remove(name string) error {
	err := os.Remove(name)
	if err != nil {
		GinkgoWriter.Printf("ERROR occurred during removing %q, error: %s\n", name, err)
	}

	return err
}

// NewVegetaCSVEncoder returns a vegeta CSV encoder.
func NewVegetaCSVEncoder(w io.Writer) vegeta.Encoder {
	return vegeta.NewCSVEncoder(w)
}

// NewCSVResultsWriter creates and returns a CSV results file and writer.
func NewCSVResultsWriter(resultsDir, fileName string, resultHeaders ...string) (*os.File, *csv.Writer, error) {
	if err := MkdirAll(resultsDir, 0o750); err != nil {
		return nil, nil, err
	}

	file, err := OpenFile(filepath.Join(resultsDir, fileName), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}

	writer := csv.NewWriter(file)

	if err = WriteCSVRecord(writer, resultHeaders); err != nil {
		return nil, nil, err
	}

	return file, writer, nil
}
