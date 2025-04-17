package database

import (
    "strconv"
    "strings"
    "sort"
    "fmt"
    "log"
    "os"
    "regexp"
    "database/sql"
)

func (c *Client) migrate() error {
	initExists, err := c.doesInitTableExist()
	if err != nil {
		return fmt.Errorf("error, when checking if database initialization is needed. Error: %v", err)
	}

	if !initExists {
		log.Println("database is not initialized, creating init table")
		err = c.createInitTable()
		if err != nil {
			return fmt.Errorf("error, when attempting to create init table. Error: %v", err)
		}
		log.Println("database initialization complete")
	}

	log.Println("checking for migrations")
	dirEntries, err := os.ReadDir(c.migrationDir)
	if err != nil {
        return fmt.Errorf("error, when attempting to read database directory. Directory: %s. Error: %v", c.migrationDir, err)
	}
	var migrationFileCandidateFileNames []string
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			migrationFileCandidateFileNames = append(migrationFileCandidateFileNames, entry.Name())
		}
	}

	migrationFiles := c.filterForMigrationFiles(migrationFileCandidateFileNames)
	var migrationsCompleted []string
	noMigrationsToProcessMessage := "no database migration files to process, skipping migrations"
	if len(migrationFiles) == 0 {
		log.Println(noMigrationsToProcessMessage)
		return nil
	} else {
		migrationsCompleted, err = c.checkForCompletedMigrations()
		if err != nil {
			return fmt.Errorf("error, when checking for completed migrations: %v", err)
		}
	}

	migrationsNeeded := c.determineMigrationsNeeded(migrationFiles, migrationsCompleted)
	migrationsNeededSorted := c.sortMigrationsNeededFiles(migrationsNeeded)
	for _, fileName := range migrationsNeededSorted {
		log.Printf("attempting to perform database migration with %s", fileName)

		filePath := fmt.Sprintf("%s/%s", c.migrationDir, fileName)
		err = c.executeSQLFile(filePath)
		if err != nil {
			return fmt.Errorf("error, when executing sql script: Filename: %s. Error: %v", fileName, err)
		}
		err = c.recordSuccessfulMigration(fileName)
		if err != nil {
			return fmt.Errorf("error, when attempting to record a successful migration. Error: %v", err)
		}
	}
	log.Println("finished database schema changes")
	return nil
}

func (c *Client) createInitTable() error {
    _, err := c.Conn.Exec(
        `CREATE TABLE init (                                                   
           id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,                    
           migration_file_name TEXT NOT NULL UNIQUE                          
        )`)
    if err != nil {
        return fmt.Errorf("error, when executing query to create init table. Error: %v", err)
    }
	return nil
}

func (c *Client) sortMigrationsNeededFiles(needed []string) []string {
	re := regexp.MustCompile(`^(\d+)`)
	sort.Slice(needed, func(i, j int) bool {
		num1, _ := strconv.Atoi(re.FindStringSubmatch(needed[i])[1])
		num2, _ := strconv.Atoi(re.FindStringSubmatch(needed[j])[1])
		return num1 < num2
	})
	return needed
}

func (c *Client) determineMigrationsNeeded(migrationFiles []string, migrationsCompleted []string) []string {
	var migrationsNeeded []string
	migrationsCompletedMap := make(map[string]bool)
	for _, value := range migrationsCompleted {
		migrationsCompletedMap[value] = true
	}
	for _, value := range migrationFiles {
		if !migrationsCompletedMap[value] {
			migrationsNeeded = append(migrationsNeeded, value)
		}
	}
	return migrationsNeeded
}

func (c *Client) filterForMigrationFiles(candidates []string) []string {
	var migrationFiles []string
	re := regexp.MustCompile(`^\d+`)
	for _, fileName := range candidates {
		if re.MatchString(fileName) {
			migrationFiles = append(migrationFiles, fileName)
		}
	}
	return migrationFiles
}

func (c *Client) recordSuccessfulMigration(fileName string) error {
	_, err := c.Conn.Exec(
		`INSERT INTO init (migration_file_name)
        VALUES (?)`,
		fileName,
	)
	if err != nil {
		return fmt.Errorf("error, when attempting to run sql command. Error: %v", err)
	}
	return nil
}

func (c *Client) checkForCompletedMigrations() (results []string, err error) {
	var rows *sql.Rows
	rows, err = c.Conn.Query(
		`SELECT migration_file_name
        FROM init`,
	)
	defer func() {
		err = rows.Err()
		if err != nil {
			err = fmt.Errorf("error, when reading rows. Error: %v", err)
		}
		rows.Close()
	}()

	if err != nil {
		return nil, fmt.Errorf("error, when attempting to retrieve pending migrations. Error: %v", err)
	}

	for rows.Next() {
		var result string
		err = rows.Scan(
			&result,
		)
		if err != nil {
			return nil, fmt.Errorf("error, when scanning for pending migrations. Error: %v", err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (c *Client) doesInitTableExist() (bool, error) {
	var result bool
	row := c.Conn.QueryRow(
        `SELECT count(*)
        FROM sqlite_master
        WHERE type='table' 
            AND name='init'`,
	)
	err := row.Scan(
		&result,
	)
	if err != nil {
		return false, fmt.Errorf("error, when checking to see if database had been initialized. Error: %v", err)
	}
	return result, nil
}

func (c *Client) executeSQLFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error, failed to read SQL file. Error: %w", err)
	}

	sql := string(content)
	queries := strings.Split(sql, ";")

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		_, err = c.Conn.Exec(query)
		if err != nil {
			return fmt.Errorf("error, failed to execute. Query: %s. Error: %v", query, err)
		}
	}

	return nil
}
