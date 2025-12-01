package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
)

const (
	integrationID = "ngf"
)

type fileToCopy struct {
	destDirName string
	srcFileName string
}

type initializeConfig struct {
	fileManager   file.OSFileManager
	fileGenerator config.Generator
	logger        logr.Logger
	podUID        string
	clusterUID    string
	copy          []fileToCopy
	plus          bool
}

func initialize(cfg initializeConfig) error {
	for _, f := range cfg.copy {
		if err := copyFile(cfg.fileManager, f.srcFileName, f.destDirName); err != nil {
			return err
		}
	}

	if !cfg.plus {
		cfg.logger.Info("Finished initializing configuration")
		return nil
	}

	depCtx := dataplane.DeploymentContext{
		InstallationID: &cfg.podUID,
		ClusterID:      &cfg.clusterUID,
		Integration:    integrationID,
	}

	depCtxFile, err := cfg.fileGenerator.GenerateDeploymentContext(depCtx)
	if err != nil {
		return fmt.Errorf("failed to generate deployment context file: %w", err)
	}

	if err := file.Write(cfg.fileManager, file.Convert(depCtxFile)); err != nil {
		return fmt.Errorf("failed to write deployment context file: %w", err)
	}

	cfg.logger.Info("Finished initializing configuration")

	return nil
}

func copyFile(osFileManager file.OSFileManager, src, dest string) error {
	srcFile, err := osFileManager.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := osFileManager.Create(filepath.Join(dest, filepath.Base(src)))
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer destFile.Close()

	if err := osFileManager.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("error copying file contents: %w", err)
	}

	if err := osFileManager.Chmod(destFile, os.FileMode(file.RegularFileModeInt)); err != nil {
		return fmt.Errorf("error setting file permissions: %w", err)
	}

	return nil
}
