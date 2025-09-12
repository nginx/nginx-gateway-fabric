package framework

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// CreateResultsDir creates and returns the name of the results directory for a test.
func CreateResultsDir(testName, version string) (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dirName := filepath.Join(filepath.Dir(pwd), "results", testName, version)

	if _, err := os.Stat(dirName); err == nil {
		if err := os.RemoveAll(dirName); err != nil {
			return "", fmt.Errorf("failed to remove existing directory %s: %w", dirName, err)
		}
	}

	return dirName, os.MkdirAll(dirName, 0o777)
}

// CreateResultsFile creates and returns the results file for a test.
func CreateResultsFile(filename string) (*os.File, error) {
	outFile, err := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
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
		return err
	}
	if ci.IsGKE {
		if _, err := fmt.Fprintf(file, "- Zone: %s\n- Instance Type: %s\n", ci.GkeZone, ci.GkeInstanceType); err != nil {
			return err
		}
	}
	return nil
}

func generatePNG(resultsDir, inputFilename, outputFilename, configFilename string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	gnuplotCfg := filepath.Join(filepath.Dir(pwd), "scripts", configFilename)

	files := fmt.Sprintf("inputfile='%s';outputfile='%s'", inputFilename, outputFilename)
	cmd := exec.CommandContext(context.Background(), "gnuplot", "-e", files, "-c", gnuplotCfg)
	cmd.Dir = resultsDir

	output, err := cmd.CombinedOutput()
	if err != nil {
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

	return reporter.Report(resultsFile)
}

// WriteContent writes basic content to the results file.
func WriteContent(resultsFile *os.File, content string) error {
	if _, err := fmt.Fprintln(resultsFile, content); err != nil {
		return err
	}

	return nil
}

// NewVegetaCSVEncoder returns a vegeta CSV encoder.
func NewVegetaCSVEncoder(w io.Writer) vegeta.Encoder {
	return vegeta.NewCSVEncoder(w)
}

// NewCSVResultsWriter creates and returns a CSV results file and writer.
func NewCSVResultsWriter(resultsDir, fileName string, resultHeaders ...string) (*os.File, *csv.Writer, error) {
	if err := os.MkdirAll(resultsDir, 0o750); err != nil {
		return nil, nil, err
	}

	file, err := os.OpenFile(filepath.Join(resultsDir, fileName), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}

	writer := csv.NewWriter(file)

	if err = writer.Write(resultHeaders); err != nil {
		return nil, nil, err
	}

	return file, writer, nil
}
