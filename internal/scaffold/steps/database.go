package steps

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/michaeldyrynda/arbor/internal/scaffold/types"
)

type DatabaseStep struct {
	name     string
	priority int
}

func NewDatabaseStep(priority int) *DatabaseStep {
	return &DatabaseStep{
		name:     "database.create",
		priority: priority,
	}
}

func (s *DatabaseStep) Name() string {
	return s.name
}

func (s *DatabaseStep) Priority() int {
	return s.priority
}

func (s *DatabaseStep) Condition(ctx types.ScaffoldContext) bool {
	return true
}

func (s *DatabaseStep) Run(ctx types.ScaffoldContext, opts types.StepOptions) error {
	dbType := ctx.Env["DB_CONNECTION"]
	dbName := ctx.Env["DB_DATABASE"]

	if dbType == "" && dbName == "" {
		dbType, dbName = s.readDbConfigFromEnv(ctx.WorktreePath)
	}

	if dbType == "" && dbName == "" {
		if opts.Verbose {
			fmt.Printf("  No database configuration found, skipping database creation.\n")
		}
		return nil
	}

	if opts.Verbose {
		fmt.Printf("  Creating database (%s)...\n", dbType)
	}

	if dbType == "sqlite" {
		return s.createSqliteDatabase(ctx, dbName, opts)
	}

	return s.createMysqlOrPgsqlDatabase(ctx, dbName, opts)
}

func (s *DatabaseStep) readDbConfigFromEnv(worktreePath string) (string, string) {
	envFile := filepath.Join(worktreePath, ".env")
	data, err := os.ReadFile(envFile)
	if err != nil {
		return "", ""
	}

	lines := string(data)
	var dbType, dbName string

	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DB_CONNECTION=") {
			dbType = strings.TrimPrefix(line, "DB_CONNECTION=")
		}
		if strings.HasPrefix(line, "DB_DATABASE=") {
			dbName = strings.TrimPrefix(line, "DB_DATABASE=")
		}
	}

	return dbType, dbName
}

func (s *DatabaseStep) createSqliteDatabase(ctx types.ScaffoldContext, dbName string, opts types.StepOptions) error {
	if dbName == "" {
		dbName = "database/database.sqlite"
	}

	dbFile := filepath.Join(ctx.WorktreePath, dbName)
	dbDir := filepath.Dir(dbFile)

	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		if opts.Verbose {
			fmt.Printf("  Creating SQLite database: %s\n", dbName)
		}

		file, err := os.Create(dbFile)
		if err != nil {
			return fmt.Errorf("creating SQLite database file: %w", err)
		}
		file.Close()
	} else {
		if opts.Verbose {
			fmt.Printf("  SQLite database already exists: %s\n", dbName)
		}
	}

	return nil
}

func (s *DatabaseStep) createMysqlOrPgsqlDatabase(ctx types.ScaffoldContext, dbName string, opts types.StepOptions) error {
	if dbName == "" {
		dbName = generateDatabaseName()
	}

	dbUser := ctx.Env["DB_USERNAME"]
	if dbUser == "" {
		dbUser = "root"
	}
	dbPass := ctx.Env["DB_PASSWORD"]
	dbHost := ctx.Env["DB_HOST"]
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := ctx.Env["DB_PORT"]
	if dbPort == "" {
		dbPort = "3306"
	}

	if opts.Verbose {
		fmt.Printf("  Generated database name: %s\n", dbName)
	}

	var createCmd *exec.Cmd
	if _, err := exec.LookPath("mysql"); err == nil {
		createCmd = exec.Command("mysql", "-u", dbUser, "-h", dbHost, "-P", dbPort)
		if dbPass != "" {
			createCmd.Args = append(createCmd.Args, fmt.Sprintf("-p%s", dbPass))
		}
		createCmd.Args = append(createCmd.Args, "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName))
	} else if _, err := exec.LookPath("psql"); err == nil {
		env := os.Environ()
		env = append(env, fmt.Sprintf("PGPASSWORD=%s", dbPass))
		createCmd = exec.Command("psql", "-U", dbUser, "-h", dbHost, "-p", dbPort, "-c", fmt.Sprintf("CREATE DATABASE \"%s\"", dbName))
		createCmd.Env = env
	}

	if createCmd != nil {
		if opts.Verbose {
			fmt.Printf("  Creating database with: %s\n", createCmd.Path)
		}
		output, err := createCmd.CombinedOutput()
		if err != nil {
			if opts.Verbose {
				fmt.Printf("  Database creation output: %s\n", string(output))
			}
			fmt.Printf("  Could not create database automatically: %v\n", err)
			fmt.Printf("  Please create database '%s' manually before running migrations.\n", dbName)
		} else {
			if opts.Verbose {
				fmt.Printf("  Database '%s' created successfully.\n", dbName)
			}
		}
	} else {
		fmt.Printf("  No MySQL or PostgreSQL client found.\n")
		fmt.Printf("  Please create database '%s' manually before running migrations.\n", dbName)
	}

	return nil
}

func generateDatabaseName() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return fmt.Sprintf("app_%s", hex.EncodeToString(bytes))
}
