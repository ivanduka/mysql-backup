package main

import (
	"fmt"
	"log"
	"os"
)
import "github.com/joho/godotenv"
import "database/sql"
import _ "github.com/go-sql-driver/mysql"

type database struct {
	dbName string
	tables []string
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("cannot load environmental variables")
	}
}

func isUserDB(db string) bool {
	systemDatabases := []string{"information_schema", "sys", "performance_schema", "mysql"}
	for _, v := range systemDatabases {
		if db == v {
			return false
		}
	}
	return true
}

func getConn(databaseName string) (*sql.DB, error) {
	connStr := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v", os.Getenv("DB_NAME"), os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), databaseName)
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

func main() {
	data, err := getDatabasesAndTables()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(data)
}
