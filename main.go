package main

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	_ "github.com/mattn/go-sqlite3"
)

// App maps to apps table
type App struct {
	id    int
	title string
}

// BaseDir is the base search path
const BaseDir = "/private/var/folders"

func findDB() (string, error) {
	var (
		file string
		re   error
	)

	s := spinner.New(spinner.CharSets[36], 1000*time.Millisecond)
	s.Prefix = "Searching database file: "
	s.Start()

	e := filepath.Walk(BaseDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "db" && strings.Contains(path, "com.apple.dock.launchpad") {
			file = path
			return nil
		}
		return nil
	})
	s.Stop()

	if e != nil {
		log.Fatal(e)
	}

	if file == "" {
		re = errors.New("Couldn't find the sqlite database file")
	}

	return file, re
}

func list(db *sql.DB) []App {
	sql := `select item_id, title from apps;`
	rows, err := db.Query(sql)

	if err != nil {
		log.Fatal(err)
	}

	defer rows.Close()

	apps := []App{}
	for rows.Next() {
		app := App{}
		if err := rows.Scan(&app.id, &app.title); err != nil {
			log.Fatal(err)
		}
		apps = append(apps, app)
	}
	// If the database is being written to ensure to check for Close
	// errors that may be returned from the driver. The query may
	// encounter an auto-commit error and be forced to rollback changes.
	rerr := rows.Close()
	if rerr != nil {
		log.Fatal(err)
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	return apps
}

func delete(db *sql.DB, ids []int) {
	idstrs := []string{}
	for _, id := range ids {
		idstrs = append(idstrs, strconv.Itoa(id))
	}

	sql := "delete from apps where item_id in (" + strings.Join(idstrs, ",") + ");"
	_, err := db.Exec(sql)

	if err != nil {
		log.Fatal(err)
	}
}

func restartDock() {
	cmd := exec.Command("/bin/sh", "-c", "killall Dock")
	err := cmd.Run()

	if err != nil {
		log.Fatal(err)
	}
}

func drawUI(db *sql.DB, apps []App) {
	if len(apps) <= 0 {
		return
	}

	titles := []string{}
	dict := map[string]int{}
	for _, app := range apps {
		titles = append(titles, app.title)
		dict[app.title] = app.id
	}

	prompt := &survey.MultiSelect{
		Message:  "Choose the items to remove",
		Options:  titles,
		Default:  titles[0],
		PageSize: 10,
	}

	ans := []string{}

	err := survey.AskOne(prompt, &ans)

	if err != nil {
		log.Fatal(err)
	}

	if len(ans) > 0 {
		ids := []int{}
		for _, title := range ans {
			ids = append(ids, dict[title])
		}

		delete(db, ids)
	}
}

func main() {
	f, err := findDB()

	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("sqlite3", f)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	apps := list(db)

	drawUI(db, apps)

	restartDock()
}
