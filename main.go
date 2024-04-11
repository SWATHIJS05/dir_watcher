package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/fsnotify.v1"
)

type File struct {
	Id               int    `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	CountOfMagicWord int    `json:"countOfMagicWord"`
	CreatedAt        string `json:"createdAt"`
	ModifiedAt       string `json:"modifiedAt"`
}

type InputData struct {
	DirectoryName string `json:"directoryName"`
	MagicWord     string `json:"magicWord"`
}

var (
	directory = "TestDirectory"
	dirpath   = "C:\\Users\\ADMIN\\Desktop\\swathi\\DirWatcher\\"
	magicword = "magic"

	timePeriod = 5 * time.Second
	signal1    = make(chan string)
	signal2    = make(chan string)
)

var (
	// Database connection parameters
	DbUser     = "root"
	DbPassword = "password"
	DbHost     = "localhost"
	DbPort     = "3306"
	DbName     = "directory_watcher"
)

func main() {
	fmt.Println("Welcome to the Directory watcher")
	err := Dbinit()
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
		return
	}
	if err := MountRoutes(); err != nil {
		log.Fatalf("Failed Mounting routes: %v", err)
		return

	}

}

func Dbinit() error {

	// Connection string
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/", DbUser, DbPassword, DbHost, DbPort)

	// Connect to MySQL server
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL server: %v", err)
		return err
	}
	defer db.Close()

	// Create database if not exists
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + DbName)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
		return err
	}

	// Connect to the created database
	dataSourceName = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", DbUser, DbPassword, DbHost, DbPort, DbName)

	db, err = sql.Open("mysql", dataSourceName)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL server: %v", err)
		return err
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS files (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		status VARCHAR(50),
		count_of_magic_word INT8,
		created_at VARCHAR(50),
		modified_at VARCHAR(50)
	)`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
		return err
	}

	log.Println("Database and table created successfully")
	return nil
}

func GetDBCon() (*sql.DB, error) {
	dataSourceName := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", DbUser, DbPassword, DbHost, DbPort, DbName)
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
		return nil, err
	}
	return db, nil

}

func MountRoutes() error {
	//REST API handling
	// Create a new Gin router
	router := gin.Default()
	router.GET("/files", GetHandler)
	router.POST("/taskStart", TaskStart)
	router.POST("/taskStop", TaskStop)
	router.POST("/configureTask", ConfigureTask)

	// Run the HTTP server on port 8080
	if err := router.Run(""); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func ConfigureTask(c *gin.Context) {
	var inputData InputData
	if err := c.BindJSON(&inputData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process the input data
	directory = inputData.DirectoryName
	magicword = inputData.MagicWord
	c.JSON(http.StatusOK, gin.H{"message": "Task Configured"})
}

func GetHandler(c *gin.Context) {
	// Execute the SQL query to fetch files data
	db, err := GetDBCon()
	if err != nil {
		log.Printf("Error establishing db connection %s", err)
		return
	}
	rows, err := db.Query("SELECT * FROM files;")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to execute query")
		return
	}
	defer rows.Close()

	// Iterate through the rows and build a response
	var files []File
	//log.Printf("Rows: %s\n", rows)
	for rows.Next() {
		var file File
		var name string
		if err := rows.Scan(&file.Id, &name, &file.Status, &file.CountOfMagicWord, &file.CreatedAt, &file.ModifiedAt); err != nil {
			log.Printf("Error: %s\n", err)
			c.String(http.StatusInternalServerError, "Failed to scan rows")
			return
		}
		names := strings.Split(name, "\\")
		file.Name = names[len(names)-1]
		files = append(files, file)
		// log.Printf("Rows: %s\n", files)
	}
	if err := rows.Err(); err != nil {
		c.String(http.StatusInternalServerError, "Error occurred while iterating rows")
		return
	}
	// Respond with the files data
	c.JSON(http.StatusOK, gin.H{"fileDetails": files})

}

func TaskStart(c *gin.Context) {
	db, err := GetDBCon()
	if err != nil {
		log.Printf("Error establishing db connection %s", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go DirWatcher(ctx, db, signal1)

	go WalkDir(ctx, db, signal2)

	// Respond with the success data
	c.JSON(http.StatusOK, gin.H{"message": "Task started successfully"})

}

func TaskStop(c *gin.Context) {
	// db, err := GetDBCon()
	// if err != nil {
	// 	log.Printf("Error establishing db connection %s", err)
	// 	return
	// }

	// signal
	go func() {
		signal1 <- "Cancelled"
		signal2 <- "Cancelled"
	}()

	// Respond with the success data
	c.JSON(http.StatusOK, gin.H{"message": "Task stopped successfully"})

}

func WalkDir(ctx context.Context, db *sql.DB, signal2 chan string) {

	log.Println("Walking through the directory")
	// Define the directory to search

	// Create a ticker that ticks every 5 minutes
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// Execute the directory search program every time the ticker ticks

	// Start the ticker loop
	for {
		select {
		case <-ticker.C:
			// Send a signal to perform the file search
			fmt.Println("Performing file search...")
			err := searchDirectory(db, dirpath+directory, magicword)
			if err != nil {
				log.Printf("Error searching directory: %v\n", err)
			}
		case sig := <-signal2:
			if sig == "Cancelled" {
				log.Println("Received the cancellation signal and stopping the task")
				ctx.Done()
				return

			}
		}

	}

}

func searchDirectory(db *sql.DB, directory, magicWord string) error {
	// Traverse the directory and search for files
	fmt.Println("Performing file search...", directory)
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v\n", path, err)
			return err
		}
		if !info.IsDir() {
			// Read file contents
			content, err := ioutil.ReadFile(path)
			if err != nil {
				log.Printf("Error reading file %s: %v\n", path, err)
				return err
			}
			// Check if the magic word exists in the file
			count := strings.Count(string(content), magicWord)
			if count != 0 {
				exists, err := recordExists(db, path)
				if err != nil {
					log.Fatal(err)
				}
				if !exists {
					if err := insertRecord(db, path); err != nil {
						log.Fatal(err)
					}

				} else {
					log.Printf("Updating the record")
					if err := updateRecord(db, path, "active", count); err != nil {
						log.Printf("Error updating file %s: %v\n", path, err)
						log.Fatal(err)
					}

				}

			}

		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}
	return nil
}

func DirWatcher(ctx context.Context, db *sql.DB, signal1 chan string) {
	// Create a new watcher instance
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Define the directory to watch
	dict := dirpath + directory

	// Add the directory to the watcher
	err = watcher.Add(dict)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Watching :", dict)

	// Start listening for events
	for {

		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Println("Event:", event)

			// Check the event type
			if event.Op&fsnotify.Create == fsnotify.Create {
				log.Println("New file created:", event.Name)
				exists, err := recordExists(db, event.Name)
				if err != nil {
					log.Fatal(err)
				}
				if !exists {
					if err := insertRecord(db, event.Name); err != nil {
						log.Fatal(err)
					}

				}

			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				log.Println("File removed:", event.Name)
				if err := updateRecord(db, event.Name, "deleted", 0); err != nil {
					log.Fatal(err)
				}

			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Error:", err)
		case sig := <-signal1:
			if sig == "Cancelled" {
				log.Println("Received the signal")
				ctx.Done()
				return

			}

		}
	}
}

func recordExists(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM files WHERE name = ?", name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func insertRecord(db *sql.DB, name string) error {
	log.Println("Someone is inserting the record")
	_, err := db.Exec("INSERT INTO files (name, created_at, modified_at, status, count_of_magic_word) VALUES (?, ?, ?, ?, ?)",
		name, time.Now(), time.Now(), "active", 0)
	return err

}

func updateRecord(db *sql.DB, name, status string, count int) error {
	log.Println("Someone is updating the record")
	_, err := db.Exec("UPDATE files SET status = ? , modified_at = ? , count_of_magic_word=? WHERE name = ?", status, time.Now(), count, name)
	return err
}
