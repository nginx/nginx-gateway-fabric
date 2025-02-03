package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/licensing"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
)

const (
	collectDeployCtxTimeout = 10 * time.Second
)

type copyFiles struct {
	destDirName  string
	srcFileNames []string
}

type initializeConfig struct {
	collector     licensing.Collector
	fileManager   file.OSFileManager
	fileGenerator config.Generator
	logger        logr.Logger
	copy          copyFiles
	plus          bool
}

func initialize(cfg initializeConfig) error {
	for _, src := range cfg.copy.srcFileNames {
		if err := copyFile(cfg.fileManager, src, cfg.copy.destDirName); err != nil {
			return err
		}
	}

	if !cfg.plus {
		cfg.logger.Info("Finished initializing configuration")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), collectDeployCtxTimeout)
	defer cancel()

	depCtx, err := cfg.collector.Collect(ctx)
	if err != nil {
		cfg.logger.Error(err, "error collecting deployment context")
	}

	cfg.logger.Info("Deployment context collected", "deployment context", depCtx)

	depCtxFile, err := cfg.fileGenerator.GenerateDeploymentContext(depCtx)
	if err != nil {
		return fmt.Errorf("failed to generate deployment context file: %w", err)
	}

	if err := file.WriteFile(cfg.fileManager, depCtxFile); err != nil {
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

	return nil
}
