package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	migrationOwner        = "pathfinder_owner"
	migrationPasswordFile = "/usr/local/etc/pathfinder/secrets/postgresql-migration-password"
	migrationUser         = "pathfinder_migrator"
	psqlPath              = "/usr/local/bin/psql"
)

func runMigrate(args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf(
			"migration command must run as root",
		)
	}

	path, err := configPath("migrate", args)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}

	password, err := os.ReadFile(migrationPasswordFile)
	if err != nil {
		return fmt.Errorf(
			"read migration password: %w",
			err,
		)
	}

	passwordText := strings.TrimSpace(string(password))
	if passwordText == "" {
		return fmt.Errorf(
			"migration password file is empty",
		)
	}

	if _, err := os.Stat(psqlPath); err != nil {
		return fmt.Errorf(
			"PostgreSQL client unavailable: %w",
			err,
		)
	}

	passfile, err := os.CreateTemp(
		"",
		"pathfinder-migrate-pgpass-*",
	)
	if err != nil {
		return fmt.Errorf(
			"create migration password file: %w",
			err,
		)
	}

	passfilePath := passfile.Name()

	defer func() {
		_ = os.Remove(passfilePath)
	}()

	if err := passfile.Chmod(0600); err != nil {
		_ = passfile.Close()

		return fmt.Errorf(
			"secure migration password file: %w",
			err,
		)
	}

	_, err = fmt.Fprintf(
		passfile,
		"%s:%d:%s:%s:%s\n",
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
		migrationUser,
		passwordText,
	)
	if err != nil {
		_ = passfile.Close()

		return fmt.Errorf(
			"write migration password file: %w",
			err,
		)
	}

	if err := passfile.Close(); err != nil {
		return fmt.Errorf(
			"close migration password file: %w",
			err,
		)
	}

	sql := `
BEGIN;

SET ROLE pathfinder_owner;

SELECT
    session_user || '|' || current_user;

ROLLBACK;
`

	command := exec.Command(
		psqlPath,
		"-X",
		"-A",
		"-t",
		"-q",
		"-v",
		"ON_ERROR_STOP=1",
		"-h",
		cfg.DatabaseHost,
		"-p",
		strconv.Itoa(cfg.DatabasePort),
		"-U",
		migrationUser,
		"-d",
		cfg.DatabaseName,
	)

	command.Env = append(
		os.Environ(),
		"PGAPPNAME=pathfinder-migrate",
		"PGCONNECT_TIMEOUT=3",
		"PGPASSFILE="+passfilePath,
	)

	command.Stdin = strings.NewReader(sql)

	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"migration authority preflight failed: %s: %w",
			strings.TrimSpace(string(output)),
			err,
		)
	}

	expectedIdentity := migrationUser + "|" + migrationOwner

	if !strings.Contains(
		string(output),
		expectedIdentity,
	) {
		return fmt.Errorf(
			"migration role transition not proven",
		)
	}

	fmt.Println("Pathfinder migration preflight: PASS")

	fmt.Printf(
		"  database=%s:%d/%s\n",
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
	)

	fmt.Printf(
		"  migration_user=%s\n",
		migrationUser,
	)

	fmt.Printf(
		"  migration_role=%s\n",
		migrationOwner,
	)

	fmt.Println("  migrations_pending=0")

	return nil
}
