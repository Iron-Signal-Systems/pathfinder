package main

import (
	"flag"
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

type migrationOptions struct {
	ConfigPath string
	DryRun     bool
}

func migrationArgs(args []string) (migrationOptions, error) {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)

	configPath := flags.String(
		"config",
		"/usr/local/etc/pathfinder/pathfinder.conf",
		"Pathfinder configuration file",
	)

	dryRun := flags.Bool(
		"dry-run",
		false,
		"execute pending migrations and roll them back",
	)

	if err := flags.Parse(args); err != nil {
		return migrationOptions{}, err
	}

	if flags.NArg() != 0 {
		return migrationOptions{}, fmt.Errorf("unexpected arguments")
	}

	return migrationOptions{
		ConfigPath: *configPath,
		DryRun:     *dryRun,
	}, nil
}

func migrationApplied(
	cfg Config,
	passfilePath string,
	item migration,
) (bool, error) {
	tableOutput, err := runPSQL(
		cfg,
		passfilePath,
		`
SET ROLE pathfinder_owner;
SELECT to_regclass('pathfinder.schema_migration') IS NOT NULL;
`,
	)
	if err != nil {
		return false, fmt.Errorf("check migration table: %w", err)
	}

	if strings.TrimSpace(tableOutput) != "t" {
		return false, nil
	}

	hashOutput, err := runPSQL(
		cfg,
		passfilePath,
		fmt.Sprintf(
			`
SET ROLE pathfinder_owner;
SELECT sha256
FROM pathfinder.schema_migration
WHERE version = %d;
`,
			item.Version,
		),
	)
	if err != nil {
		return false, fmt.Errorf(
			"check migration %d: %w",
			item.Version,
			err,
		)
	}

	appliedHash := strings.TrimSpace(hashOutput)

	if appliedHash == "" {
		return false, nil
	}

	if appliedHash != item.SHA256 {
		return false, fmt.Errorf(
			"migration %04d checksum mismatch: database=%s embedded=%s",
			item.Version,
			appliedHash,
			item.SHA256,
		)
	}

	return true, nil
}

func migrationSQL(item migration, dryRun bool) string {
	finalStatement := "COMMIT;"
	if dryRun {
		finalStatement = "ROLLBACK;"
	}

	return fmt.Sprintf(
		`
BEGIN;

SELECT pg_advisory_xact_lock(
    hashtextextended('pathfinder-schema-migration', 0)
);

SET ROLE pathfinder_owner;

%s

INSERT INTO pathfinder.schema_migration (
    version,
    name,
    sha256,
    applied_by
)
VALUES (
    %d,
    '%s',
    '%s',
    session_user
);

%s
`,
		item.SQL,
		item.Version,
		item.Name,
		item.SHA256,
		finalStatement,
	)
}

func migrationPreflight(
	cfg Config,
	passfilePath string,
) error {
	output, err := runPSQL(
		cfg,
		passfilePath,
		`
BEGIN;
SET ROLE pathfinder_owner;
SELECT session_user || '|' || current_user;
ROLLBACK;
`,
	)
	if err != nil {
		return fmt.Errorf(
			"migration authority preflight failed: %w",
			err,
		)
	}

	expectedIdentity := migrationUser + "|" + migrationOwner

	if !strings.Contains(output, expectedIdentity) {
		return fmt.Errorf("migration role transition not proven")
	}

	return nil
}

func runMigrate(args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("migration command must run as root")
	}

	options, err := migrationArgs(args)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(options.ConfigPath)
	if err != nil {
		return err
	}

	password, err := os.ReadFile(migrationPasswordFile)
	if err != nil {
		return fmt.Errorf("read migration password: %w", err)
	}

	passwordText := strings.TrimSpace(string(password))
	if passwordText == "" {
		return fmt.Errorf("migration password file is empty")
	}

	if _, err := os.Stat(psqlPath); err != nil {
		return fmt.Errorf("PostgreSQL client unavailable: %w", err)
	}

	passfile, err := os.CreateTemp("", "pathfinder-migrate-pgpass-*")
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

	if err := migrationPreflight(cfg, passfilePath); err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	pending := 0
	applied := 0

	for _, item := range migrations {
		isApplied, err := migrationApplied(
			cfg,
			passfilePath,
			item,
		)
		if err != nil {
			return err
		}

		if isApplied {
			fmt.Printf(
				"Migration %04d %s: ALREADY_APPLIED\n",
				item.Version,
				item.Name,
			)
			continue
		}

		pending++

		if options.DryRun {
			fmt.Printf(
				"Migration %04d %s: DRY_RUN\n",
				item.Version,
				item.Name,
			)
		} else {
			fmt.Printf(
				"Migration %04d %s: APPLY\n",
				item.Version,
				item.Name,
			)
		}

		if _, err := runPSQL(
			cfg,
			passfilePath,
			migrationSQL(item, options.DryRun),
		); err != nil {
			return fmt.Errorf(
				"migration %04d %s failed: %w",
				item.Version,
				item.Name,
				err,
			)
		}

		if !options.DryRun {
			applied++
		}
	}

	fmt.Println("Pathfinder migration: PASS")
	fmt.Printf(
		"  database=%s:%d/%s\n",
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
	)
	fmt.Printf("  migration_user=%s\n", migrationUser)
	fmt.Printf("  migration_role=%s\n", migrationOwner)
	fmt.Printf("  migrations_pending=%d\n", pending)
	fmt.Printf("  migrations_applied=%d\n", applied)
	fmt.Printf("  dry_run=%t\n", options.DryRun)

	return nil
}

func runPSQL(
	cfg Config,
	passfilePath string,
	sql string,
) (string, error) {
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
		return "", fmt.Errorf(
			"%s: %w",
			strings.TrimSpace(string(output)),
			err,
		)
	}

	return strings.TrimSpace(string(output)), nil
}
