package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ArtifactsDir         string
	DatabaseHost         string
	DatabaseName         string
	DatabasePasswordFile string
	DatabasePort         int
	DatabaseUser         string
	LogDir               string
	ReadinessListen      string
	StateDir             string
}

func checkDatabase(cfg Config) error {
	connection, err := net.DialTimeout(
		"tcp",
		databaseAddress(cfg),
		3*time.Second,
	)
	if err != nil {
		return fmt.Errorf(
			"database endpoint %s unavailable: %w",
			databaseAddress(cfg),
			err,
		)
	}
	return connection.Close()
}

func configPath(command string, args []string) (string, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	path := flags.String(
		"config",
		"/usr/local/etc/pathfinder/pathfinder.conf",
		"Pathfinder configuration file",
	)
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("unexpected arguments")
	}
	return *path, nil
}

func databaseAddress(cfg Config) string {
	return net.JoinHostPort(
		cfg.DatabaseHost,
		strconv.Itoa(cfg.DatabasePort),
	)
}

func loadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	known := map[string]bool{
		"artifacts_dir":          true,
		"database_host":          true,
		"database_name":          true,
		"database_password_file": true,
		"database_port":          true,
		"database_user":          true,
		"log_dir":                true,
		"readiness_listen":       true,
		"state_dir":              true,
	}
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return Config{}, fmt.Errorf(
				"config line %d: expected key=value",
				lineNumber,
			)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !known[key] {
			return Config{}, fmt.Errorf(
				"config line %d: unknown key %q",
				lineNumber,
				key,
			)
		}
		if value == "" {
			return Config{}, fmt.Errorf(
				"config line %d: empty value for %q",
				lineNumber,
				key,
			)
		}
		if _, exists := values[key]; exists {
			return Config{}, fmt.Errorf(
				"config line %d: duplicate key %q",
				lineNumber,
				key,
			)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	for key := range known {
		if _, exists := values[key]; !exists {
			return Config{}, fmt.Errorf(
				"required config key missing: %s",
				key,
			)
		}
	}
	port, err := strconv.Atoi(values["database_port"])
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf(
			"database_port must be between 1 and 65535",
		)
	}
	cfg := Config{
		ArtifactsDir:         values["artifacts_dir"],
		DatabaseHost:         values["database_host"],
		DatabaseName:         values["database_name"],
		DatabasePasswordFile: values["database_password_file"],
		DatabasePort:         port,
		DatabaseUser:         values["database_user"],
		LogDir:               values["log_dir"],
		ReadinessListen:      values["readiness_listen"],
		StateDir:             values["state_dir"],
	}
	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateConfig(cfg Config) error {
	for _, directory := range []string{
		cfg.ArtifactsDir,
		cfg.LogDir,
		cfg.StateDir,
	} {
		if !filepath.IsAbs(directory) {
			return fmt.Errorf(
				"runtime directory must be absolute: %s",
				directory,
			)
		}
		info, err := os.Stat(directory)
		if err != nil {
			return fmt.Errorf(
				"runtime directory %s: %w",
				directory,
				err,
			)
		}
		if !info.IsDir() {
			return fmt.Errorf(
				"runtime path is not a directory: %s",
				directory,
			)
		}
	}
	if !filepath.IsAbs(cfg.DatabasePasswordFile) {
		return fmt.Errorf(
			"database_password_file must be absolute",
		)
	}
	info, err := os.Stat(cfg.DatabasePasswordFile)
	if err != nil {
		return fmt.Errorf(
			"database password file: %w",
			err,
		)
	}
	if info.Mode().Perm() != 0640 {
		return fmt.Errorf(
			"database password file mode is %04o; expected 0640",
			info.Mode().Perm(),
		)
	}
	password, err := os.ReadFile(cfg.DatabasePasswordFile)
	if err != nil {
		return fmt.Errorf(
			"read database password: %w",
			err,
		)
	}
	if strings.TrimSpace(string(password)) == "" {
		return fmt.Errorf(
			"database password file is empty",
		)
	}
	if err := validateReadinessAddress(cfg.ReadinessListen); err != nil {
		return err
	}
	return checkDatabase(cfg)
}

func validateReadinessAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf(
			"invalid readiness_listen: %w",
			err,
		)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf(
			"readiness_listen must use a loopback IP address",
		)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf(
			"readiness_listen port must be between 1 and 65535",
		)
	}
	return nil
}
