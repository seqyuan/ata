package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TaskStatus represents the current status of a task for monitoring
type TaskStatus struct {
	subJobNum int
	status    string
	exitCode  sql.NullInt64
	starttime sql.NullString
	endtime   sql.NullString
}

// formatTimeShort formats time string to MM-DD HH:MM format
// Input: "2006-01-02 15:04:05" -> Output: "01-02 15:04"
func formatTimeShort(timeStr string) string {
	if timeStr == "" || timeStr == "-" {
		return "-"
	}

	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t.Format("01-02 15:04")
		}
	}

	// Fallback: manual extraction
	if strings.Contains(timeStr, "T") {
		parts := strings.Split(timeStr, "T")
		if len(parts) >= 2 {
			datePart := parts[0]
			timePart := strings.Split(parts[1], ":")
			if len(timePart) >= 2 {
				dateParts := strings.Split(datePart, "-")
				if len(dateParts) >= 3 {
					return fmt.Sprintf("%s-%s %s:%s", dateParts[1], dateParts[2], timePart[0], timePart[1])
				}
			}
		}
	} else {
		parts := strings.Fields(timeStr)
		if len(parts) >= 2 {
			dateParts := strings.Split(parts[0], "-")
			timeParts := strings.Split(parts[1], ":")
			if len(dateParts) >= 3 && len(timeParts) >= 2 {
				return fmt.Sprintf("%s-%s %s:%s", dateParts[1], dateParts[2], timeParts[0], timeParts[1])
			}
		}
	}

	return timeStr
}

// MonitorTaskStatus monitors database and outputs task status changes to log file
func MonitorTaskStatus(ctx context.Context, dbObj *MySql, shellPath string, command string, isResume bool, finishedBeforeStart int) {
	logFilePath := shellPath + ".log"

	fileExists := false
	if _, err := os.Stat(logFilePath); err == nil {
		fileExists = true
	}

	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Error opening log file %s: %v", logFilePath, err)
		return
	}
	defer logFile.Close()

	var logMutex sync.Mutex

	// Write command header
	if command != "" {
		logMutex.Lock()
		if fileExists {
			fmt.Fprintf(logFile, "\n")
		}
		fmt.Fprintf(logFile, "%s\n", command)
		if isResume {
			var totalCount int
			dbObj.Db.QueryRow("SELECT COUNT(*) FROM job").Scan(&totalCount)
			fmt.Fprintf(logFile, "[RESTART] Resuming: %d/%d tasks completed, %d remaining\n",
				finishedBeforeStart, totalCount, totalCount-finishedBeforeStart)
		}
		fmt.Fprintf(logFile, "\n")
		logFile.Sync()
		logMutex.Unlock()
	}

	lastStatus := make(map[int]TaskStatus)
	headerPrinted := false

	updateLogFile := func() {
		rows, err := dbObj.Db.Query(`
			SELECT subJob_num, status, exitCode, starttime, endtime
			FROM job
			ORDER BY subJob_num
		`)
		if err != nil {
			log.Printf("Error querying task status: %v", err)
			return
		}
		defer rows.Close()

		if !headerPrinted {
			logMutex.Lock()
			fmt.Fprintf(logFile, "%-6s %-10s %-8s %-12s\n", "task", "status", "exitcode", "time")
			logFile.Sync()
			logMutex.Unlock()
			headerPrinted = true
		}

		currentStatus := make(map[int]TaskStatus)
		for rows.Next() {
			var ts TaskStatus
			err := rows.Scan(&ts.subJobNum, &ts.status, &ts.exitCode, &ts.starttime, &ts.endtime)
			if err != nil {
				log.Printf("Error scanning task status: %v", err)
				continue
			}
			currentStatus[ts.subJobNum] = ts

			// Skip Pending tasks
			if ts.status == "Pending" {
				continue
			}

			last, exists := lastStatus[ts.subJobNum]
			if !exists {
				// New non-pending task
				outputTaskStatus(logFile, &logMutex, ts)
			} else if last.status != ts.status ||
				(ts.endtime.Valid && (!last.endtime.Valid || last.endtime.String != ts.endtime.String)) {
				// Status changed or endtime updated
				outputTaskStatus(logFile, &logMutex, ts)
			}
		}

		lastStatus = currentStatus
	}

	// Initial update
	updateLogFile()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final update before exit
			updateLogFile()
			return
		case <-ticker.C:
			updateLogFile()
		}
	}
}

func outputTaskStatus(logFile *os.File, logMutex *sync.Mutex, ts TaskStatus) {
	taskNumStr := fmt.Sprintf("%04d", ts.subJobNum)

	var exitCodeStr string
	if ts.exitCode.Valid {
		exitCodeStr = strconv.FormatInt(ts.exitCode.Int64, 10)
	} else {
		exitCodeStr = "-"
	}

	var timeStr string
	if ts.endtime.Valid {
		timeStr = formatTimeShort(ts.endtime.String)
	} else if ts.starttime.Valid {
		timeStr = formatTimeShort(ts.starttime.String)
	} else {
		timeStr = "-"
	}

	logMutex.Lock()
	defer logMutex.Unlock()
	fmt.Fprintf(logFile, "%-6s %-10s %-8s %-12s\n", taskNumStr, ts.status, exitCodeStr, timeStr)
	logFile.Sync()
}
