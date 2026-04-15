package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"github.com/akamensky/argparse"
	_ "github.com/mattn/go-sqlite3"
	"github.com/seqyuan/ata/pkg/gpool"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type MySql struct {
	Db *sql.DB
}

func (sqObj *MySql) Crt_tb() {
	// create table if not exists
	sql_job_table := `
	CREATE TABLE IF NOT EXISTS job(
		Id INTEGER NOT NULL PRIMARY KEY,
		subJob_num INTEGER UNIQUE NOT NULL,
		shellPath	TEXT,
		status	TEXT,
		exitCode	integer,
		retry	integer, 
		starttime	datetime,
		endtime	datetime 
	);
	`
	_, err := sqObj.Db.Exec(sql_job_table)
	if err != nil {
		panic(err)
	}
}

type jobStatusType string

// These are project or module type.
const (
	J_pending  jobStatusType = "Pending"
	J_failed   jobStatusType = "Failed"
	J_running  jobStatusType = "Running"
	J_finished jobStatusType = "Finished"
)

func CheckCount(rows *sql.Rows) (count int) {
	count = 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		panic(err)
	}
	return count
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func GenerateShell(shellPath, content string) {
	fi, err := os.Create(shellPath)
	if err != nil {
		panic(err)
	}
	defer fi.Close()

	content = strings.TrimRight(content, "\n")
	signPath := shellQuote(shellPath + ".sign")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -e
printf '========== start at : %%s ==========\n' "$(date '+%%Y/%%m/%%d %%H:%%M:%%S')"
%s
printf '========== end at : %%s ==========\n' "$(date '+%%Y/%%m/%%d %%H:%%M:%%S')"
printf '%%s\n' LLAP 1>&2
printf '%%s\n' LLAP > %s
`, content, signPath)

	_, err = fi.Write([]byte(script))
	CheckErr(err)
}

func Creat_tb(shell_path string, line_unit int) (dbObj *MySql) {
	shellAbsName, _ := filepath.Abs(shell_path)
	dbpath := shellAbsName + ".db"
	subShellPath := shellAbsName + ".shell"

	err := os.MkdirAll(subShellPath, 0777)
	CheckErr(err)

	conn, err := sql.Open("sqlite3", dbpath)
	CheckErr(err)
	dbObj = &MySql{Db: conn}
	dbObj.Crt_tb()

	tx, _ := dbObj.Db.Begin()
	defer tx.Rollback()
	insert_job, err := tx.Prepare("INSERT INTO job(subJob_num, shellPath, status, retry) values(?,?,?,?)")
	CheckErr(err)

	f, err := os.Open(shellAbsName)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	buf := bufio.NewReader(f)

	ii := 0
	var cmd_l string = ""
	N := 0

	writeTask := func(subJobNum int, command string) {
		var existingID int
		err := tx.QueryRow("select Id from job where subJob_num = ?", subJobNum).Scan(&existingID)
		if err == nil {
			return
		}
		if err != sql.ErrNoRows {
			CheckErr(err)
		}

		command = strings.TrimRight(command, "\n")
		subShell := subShellPath + "/task_" + strings.Repeat("0", 4-len(strconv.Itoa(subJobNum))) + strconv.Itoa(subJobNum) + ".sh"
		GenerateShell(subShell, command)
		_, err = insert_job.Exec(subJobNum, subShell, J_pending, 0)
		CheckErr(err)
	}

	for {
		line, err := buf.ReadString('\n')
		if err != nil && err != io.EOF {
			CheckErr(err)
		}
		if len(line) == 0 && err == io.EOF {
			break
		}

		if ii == 0 {
			cmd_l = line
			ii++
		} else if ii < line_unit {
			cmd_l = cmd_l + line
			ii++
		} else {
			N++
			writeTask(N, cmd_l)

			ii = 1
			cmd_l = line
		}

		if err == io.EOF {
			break
		}
	}

	if ii > 0 {
		N++
		writeTask(N, cmd_l)
	}

	err = tx.Commit()
	CheckErr(err)
	return
}

func RecoverBySign(dbObj *MySql) {
	rows, err := dbObj.Db.Query("select subJob_num, shellPath from job where status!=?", J_finished)
	CheckErr(err)
	defer rows.Close()

	type recoveredTask struct {
		subJobNum int
		endtime   string
	}

	var updates []recoveredTask
	for rows.Next() {
		var subJobNum int
		var shellPath string
		err := rows.Scan(&subJobNum, &shellPath)
		CheckErr(err)
		signPath := shellPath + ".sign"
		if info, err := os.Stat(signPath); err == nil {
			updates = append(updates, recoveredTask{
				subJobNum: subJobNum,
				endtime:   info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}

	for _, task := range updates {
		_, err := dbObj.Db.Exec("UPDATE job set status=?, endtime=?, exitCode=0 where subJob_num=?", J_finished, task.endtime, task.subJobNum)
		CheckErr(err)
	}
}

func GetNeed2Run(dbObj *MySql) []int {
	//need2run := make(map[int]int)
	tx, _ := dbObj.Db.Begin()
	defer tx.Rollback()

	rows, err := tx.Query("select subJob_num from job where Status!=?", J_finished)
	CheckErr(err)
	defer rows.Close()
	var subJob_num int

	need2run_N := CheckCount(rows)
	need2run := make([]int, need2run_N)

	ii := 0
	rows2, err := tx.Query("select subJob_num from job where Status!=?", J_finished)
	CheckErr(err)
	defer rows2.Close()
	for rows2.Next() {
		err = rows2.Scan(&subJob_num)
		CheckErr(err)
		need2run[ii] = subJob_num
		ii++
	}
	return need2run
}

func IlterCommand(dbObj *MySql, thred int, need2run []int) {
	pool := gpool.New(thred)
	var writeMu sync.Mutex

	for _, N := range need2run {
		pool.Add(1)
		go RunCommand(N, pool, dbObj, &writeMu)
	}

	pool.Wait()
}

func RunCommand(N int, pool *gpool.Pool, dbObj *MySql, writeMu *sync.Mutex) {
	defer pool.Done()

	var subShellPath string
	err := dbObj.Db.QueryRow("select shellPath from job where subJob_num = ?", N).Scan(&subShellPath)
	CheckErr(err)

	now := time.Now().Format("2006-01-02 15:04:05")
	writeMu.Lock()
	_, err = dbObj.Db.Exec("UPDATE job set status=?, starttime=?, endtime=NULL, exitCode=NULL where subJob_num=?", J_running, now, N)
	writeMu.Unlock()
	CheckErr(err)

	defaultFailedCode := 1
	cmd := exec.Command("bash", subShellPath)
	// 其他程序stdout stderr改到当前目录pwd
	sho, err := os.OpenFile(fmt.Sprintf("%s.o", subShellPath), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	CheckErr(err)
	defer sho.Close()
	she, err := os.OpenFile(fmt.Sprintf("%s.e", subShellPath), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	CheckErr(err)
	defer she.Close()
	Owriter := io.MultiWriter(sho)
	Ewriter := io.MultiWriter(she)
	cmd.Stdout = Owriter
	cmd.Stderr = Ewriter
	err = cmd.Run() //blocks until sub process is complete
	//CheckErr(err)

	var exitCode int

	if err != nil {
		// try to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		} else {
			exitCode = defaultFailedCode
		}
	} else {
		// success, exitCode should be 0 if go is ok
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}

	//var lock sync.Mutex //互斥锁
	//lock.Lock()
	writeMu.Lock()
	now = time.Now().Format("2006-01-02 15:04:05")
	if exitCode == 0 {
		//update_job_end.Exec(J_finished, now, N)
		_, err = dbObj.Db.Exec("UPDATE job set status=?, endtime=?, exitCode=? where subJob_num=?", J_finished, now, exitCode, N)

	} else {
		_, err = dbObj.Db.Exec("UPDATE job set status=?, endtime=?, exitCode=? where subJob_num=?", J_failed, now, exitCode, N)

	}

	writeMu.Unlock()

	//err = tx.Commit()
	CheckErr(err)
}

func CheckExitCode(dbObj *MySql) {
	tx, _ := dbObj.Db.Begin()
	defer tx.Rollback()

	var totalCount int
	err := tx.QueryRow("select count(*) from job").Scan(&totalCount)
	CheckErr(err)

	var successCount int
	err = tx.QueryRow("select count(*) from job where exitCode=0").Scan(&successCount)
	CheckErr(err)

	var errorCount int
	err = tx.QueryRow("select count(*) from job where exitCode!=0").Scan(&errorCount)
	CheckErr(err)

	rows12, err := tx.Query("select subJob_num, shellPath from job where exitCode!=0")
	CheckErr(err)
	defer rows12.Close()

	exitCode := 0
	os.Stderr.WriteString(fmt.Sprintf("All works: %v\nSuccessed: %v\nError: %v\n", totalCount, successCount, errorCount))
	if errorCount > 0 {
		exitCode = 1
		os.Stderr.WriteString("Err Shells:\n")
	}

	var subJob_num int
	var shellPath string
	for rows12.Next() {
		err := rows12.Scan(&subJob_num, &shellPath)
		CheckErr(err)
		os.Stderr.WriteString(fmt.Sprintf("%v\t%s\n", subJob_num, shellPath))
	}

	os.Exit(exitCode)
}

func getFinishedCount(dbObj *MySql) int {
	var count int
	err := dbObj.Db.QueryRow("SELECT COUNT(*) FROM job WHERE status=?", J_finished).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

var documents string = `任务并发程序 parallel task v1.6.2`

func CheckErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	parser := argparse.NewParser("ata", documents)
	opt_i := parser.String("i", "infile", &argparse.Options{Required: true, Help: "Input shell command file (one command per line or grouped by -l)"})
	opt_l := parser.Int("l", "line", &argparse.Options{Default: 1, Help: "Number of lines to group as one task (default: 1)"})
	opt_t := parser.Int("t", "thread", &argparse.Options{Default: 1, Help: "Max concurrent tasks to run (default: 1)"})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		return
	}
	if *opt_l <= 0 {
		log.Fatal("-l must be >= 1")
	}
	if *opt_t <= 0 {
		log.Fatal("-t must be >= 1")
	}

	dbObj := Creat_tb(*opt_i, *opt_l)
	RecoverBySign(dbObj)
	need2run := GetNeed2Run(dbObj)

	// Start monitor goroutine to write input.sh.log
	shellAbsName, _ := filepath.Abs(*opt_i)
	command := strings.Join(os.Args, " ")
	finishedCount := getFinishedCount(dbObj)
	isResume := finishedCount > 0

	ctx, cancel := context.WithCancel(context.Background())
	var monitorWg sync.WaitGroup
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()
		MonitorTaskStatus(ctx, dbObj, shellAbsName, command, isResume, finishedCount)
	}()

	IlterCommand(dbObj, *opt_t, need2run)
	cancel()
	monitorWg.Wait()

	CheckExitCode(dbObj)
}
