package main

import (
	"fmt"
	"log"
	"os"
)
import "github.com/joho/godotenv"
import "database/sql"
import _ "github.com/go-sql-driver/mysql"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("cannot load environmental variables")
	}

	connStr := fmt.Sprintf("%v:%v@tcp(%v:%v)/",
		os.Getenv("DB_NAME"), os.Getenv("DB_PASS"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	fmt.Println(connStr)

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	result, err := db.Query("SHOW DATABASES;")
	if err != nil {
		log.Fatal(err)
	}

	var databases []string
	for result.Next() {
		var database string
		err = result.Scan(&database)
		if err != nil {
			log.Fatal(err)
		}
		databases = append(databases, database)
	}
	fmt.Println(databases)
}
