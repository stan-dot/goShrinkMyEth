package backup

import (
	"log"
	"os/exec"
)

func backup() {

	// Define paths to the database and backup files

	originalDB := "/path/to/pb_data/data.db"
	backupDB := "/path/to/backup"

	// Prepare the command
	cmd := exec.Command("sqlite3", originalDB, ".backup", backupDB)

	// Run the command and capture any errors
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Backup failed: %v", err)
	}

	log.Println("Backup completed successfully.")
}
