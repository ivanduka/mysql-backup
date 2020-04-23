package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)
import "github.com/joho/godotenv"
import "database/sql"
import _ "github.com/go-sql-driver/mysql"
import "github.com/sendgrid/sendgrid-go"
import "github.com/sendgrid/sendgrid-go/helpers/mail"

var s *settings

type settings struct {
	dbUser            string
	dbPass            string
	dbHost            string
	dbPort            string
	mysqlDumpDir      string
	saveDir           string
	sevenZipPath      string
	sendgridAPIKey    string
	emailToAddress    string
	emailToName       string
	emailFromName     string
	emailFromAddress  string
	skipDatabasesList []string
}

type database struct {
	dbName string
	tables []string
}

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("cannot load environmental variables")
	}

	array := strings.Split(os.Getenv("SKIP_DATABASES"), ",")
	for i, dirty := range array {
		array[i] = strings.TrimSpace(dirty)
	}

	s = &settings{
		dbUser:            os.Getenv("DB_USER"),
		dbPass:            os.Getenv("DB_PASS"),
		dbHost:            os.Getenv("DB_HOST"),
		dbPort:            os.Getenv("DB_PORT"),
		mysqlDumpDir:      os.Getenv("MYSQLDUMP_DIR"),
		saveDir:           os.Getenv("SAVE_FOLDER"),
		sevenZipPath:      os.Getenv("7ZIP_PATH"),
		sendgridAPIKey:    os.Getenv("SENDGRID_API_KEY"),
		emailToName:       os.Getenv("EMAIL_TO_NAME"),
		emailToAddress:    os.Getenv("EMAIL_TO_ADDRESS"),
		emailFromName:     os.Getenv("EMAIL_FROM_NAME"),
		emailFromAddress:  os.Getenv("EMAIL_FROM_EMAIL"),
		skipDatabasesList: array,
	}
}

func isUserDB(db string) bool {
	for _, v := range s.skipDatabasesList {
		if db == v {
			return false
		}
	}
	return true
}

func getConn(databaseName string) (*sql.DB, error) {
	connStr := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v", s.dbUser, s.dbPass, s.dbHost, s.dbPort, databaseName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getDatabases() ([]string, error) {
	db, err := getConn("")
	if err != nil {
		return nil, err
	}

	result, err := db.Query("SHOW databases;")
	if err != nil {
		return nil, err
	}

	var databases []string
	for result.Next() {
		var database string
		err = result.Scan(&database)
		if err != nil {
			return nil, err
		}
		if isUserDB(database) {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

func getTables(database string) ([]string, error) {
	localDb, err := getConn(database)
	if err != nil {
		return nil, err
	}
	result, err := localDb.Query("SHOW tables;")
	if err != nil {
		return nil, err
	}

	var tables []string
	for result.Next() {
		var table string
		err = result.Scan(&table)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, nil
}

func getDatabasesAndTables() ([]database, error) {
	databasesNames, err := getDatabases()
	if err != nil {
		return nil, fmt.Errorf("cannot get DBs: %v", err)
	}

	var databases []database
	for _, databaseName := range databasesNames {
		tables, err := getTables(databaseName)
		if err != nil {
			return nil, fmt.Errorf("cannot get tables for %v: %v", databaseName, err)
		}
		databases = append(databases, database{
			dbName: databaseName,
			tables: tables,
		})
	}
	return databases, nil
}

func saveDump(db, table, dir string) (string, error) {
	saveName := fmt.Sprintf("%v-%v.sql", db, table)
	savePath := filepath.Join(dir, saveName)
	args := []string{"-u", s.dbUser, "--password=" + s.dbPass, "--host", s.dbHost, db, table, "--skip-lock-tables",
		"--result-file", savePath, "--default-character-set=utf8", "--no-create-db",
		"--skip-add-drop-table", "--protocol=tcp", "--single-transaction", "--quick"}
	mysqldumpPath := filepath.Join(s.mysqlDumpDir, "mysqldump")
	cmd := exec.Command(mysqldumpPath, args...)
	cmd.Dir = s.mysqlDumpDir
	out, err := cmd.CombinedOutput()
	annoyingWarning := "mysqldump: [Warning] Using a password on the command line interface can be insecure."
	result := strings.TrimSpace(strings.Replace(string(out), annoyingWarning, "", -1))
	return result, err
}

func archiveFolder(folderName string) (string, error) {
	args := []string{"a", folderName, folderName, "-mx9", "-t7z", "-sdel", "-bb1", "-slp", "-myx9", "-mmt=24"}
	cmd := exec.Command(s.sevenZipPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func saveDumps() (string, error) {
	databases, err := getDatabasesAndTables()
	if err != nil {
		return "", err
	}

	saveSubDir := filepath.Join(s.saveDir, time.Now().Format("2006-01-02_15-04-05"))
	_ = os.Mkdir(saveSubDir, os.ModePerm)

	var b strings.Builder

	for _, db := range databases {
		for _, table := range db.tables {
			str, err := saveDump(db.dbName, table, saveSubDir)
			_, _ = fmt.Fprintln(&b, str)
			if err != nil {
				return b.String(), err
			}
		}
	}

	str, err := archiveFolder(saveSubDir)
	b.WriteString(str)

	return b.String(), err
}

func sendEmail(text string, err error) {
	from := mail.NewEmail(s.emailFromName, s.emailFromAddress)
	subject := "Backup OK"
	if err != nil {
		subject = fmt.Sprintf("Backup Errored: %v", err)
	}

	htmlContent := strings.ReplaceAll(strings.TrimSpace(text), "\n", "<br>")
	to := mail.NewEmail(s.emailToName, s.emailToAddress)
	message := mail.NewSingleEmail(from, subject, to, ".", htmlContent+"<br>")
	client := sendgrid.NewSendClient(s.sendgridAPIKey)
	response, err := client.Send(message)
	if err != nil {
		log.Println(err)
	} else {
		fmt.Println("EMAIL:")
		fmt.Println("Status Code: ", response.StatusCode)
		fmt.Println("Body: ", response.Body)
		fmt.Println("Headers: ", response.Headers)
	}
}

func main() {
	out, err := saveDumps()
	if err != nil {
		fmt.Println("ERROR:", err)
	} else {
		fmt.Print("OK (no errors). Log:\n\n")
		fmt.Println(out)
	}

	sendEmail(out, err)
}
