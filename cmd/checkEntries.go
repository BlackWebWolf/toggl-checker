package cmd

import (
	"database/sql"
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	toggl "github.com/BlackWebWolf/toggl-go"
)

// checkEntriesCmd represents the checkEntries command
var checkEntriesCmd = &cobra.Command{
	Use:   "checkEntries",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		check_entries(30)
	},
}

func init() {
	rootCmd.AddCommand(checkEntriesCmd)

	// Here you will define your flags and configuration settings.
	checkEntriesCmd.PersistentFlags().StringP("days", "d", "30", "Amount of days to check")

}

type TimeEntry struct {
	id          int
	user        string
	description string
	project     string
	client      string
	duration    int64
	billable    bool
	date        int64
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func check_entries(days int) {

	days = -days
	token := os.Getenv("API_TOKEN")
	weekReport := getTogglReportData(token, days)
	entries := constructTimeEntries(weekReport)

	//fmt.Println(entries)
	db, err := sql.Open("sqlite3", "database/time_entries.db")
	checkErr(err)
	prepare_database(db)
	defer db.Close()

	changes := checkForChanges(entries, db)

	insert_entries(entries, db)
	delete_entries(db, days)
	if changes != nil {
		fmt.Println("Something has changed, please double-check")

		for _, change := range changes {
			tm := time.Unix(change.date, 0)
			entry := fmt.Sprintf(
				"Description: %s, project: %s, client: %s, billable: %t, date: %s",
				change.description,
				change.project,
				change.client,
				change.billable,
				tm.Format("2006-01-02"),
			)
			fmt.Println(entry)

		}
	} else {
		fmt.Println("Finished checking, nothing suspicious")
	}

}

func prepare_database(db *sql.DB) {
	if _, err := os.Stat("database/time_entries.db"); err != nil {
		os.MkdirAll("./database", 0755)
		os.Create("database/time_entries.db")
		_, err := db.Exec(
			"create table if not exists timeEntries" +
				" (id integer NOT NULL PRIMARY KEY, " +
				"user text, " +
				"description text, " +
				"project text, " +
				"client text, " +
				"duration int, " +
				"billable text, " +
				"date_of_entry int)",
		)
		checkErr(err)
	}
}



func insert_entries(entries []TimeEntry, db *sql.DB) {

	tx, err := db.Begin()
	checkErr(err)

	for _, entry := range entries {
		stmt, err := tx.Prepare("insert or replace into timeEntries (" +
			"id," +
			" user, " +
			"description, " +
			"project, " +
			"client, " +
			"duration, " +
			"billable, " +
			"date_of_entry) " +
			"values(?, ?, ?, ?, ?, ?, ?, ?)",
		)
		checkErr(err)
		defer stmt.Close()
		_, err = stmt.Exec(
			entry.id,
			entry.user,
			entry.description,
			entry.project,
			entry.client,
			entry.duration,
			entry.billable,
			entry.date,
		)
		checkErr(err)

	}
	tx.Commit()
}

func checkForChanges(entries []TimeEntry, db *sql.DB) []TimeEntry {
	var changedEntries []TimeEntry
	for _, entry := range entries {
		stmt, err := db.Prepare("select * from timeEntries where id = ?")
		if err != nil {
			log.Fatal(err)
		}
		defer stmt.Close()
		var test TimeEntry
		err = stmt.QueryRow(entry.id).Scan(
			&test.id,
			&test.user,
			&test.description,
			&test.project,
			&test.client,
			&test.duration,
			&test.billable,
			&test.date,
		)
		if err == nil {
			if test.billable != entry.billable {
				changedEntries = append(changedEntries, test)
			}
		}
	}

	return changedEntries

}

func delete_entries(db *sql.DB, days int) {
	stmt, err := db.Prepare("DELETE FROM timeEntries where date_of_entry <= ?")
	checkErr(err)
	defer stmt.Close()
	t_now := time.Now()
	t_week := t_now.AddDate(0, 0, days)
	_, err = stmt.Exec(t_week.Unix())
	checkErr(err)

}

func constructTimeEntries(week_report toggl.DetailedReport) (entries []TimeEntry) {
	var entry TimeEntry
	for _, val := range week_report.Data {
		entry = TimeEntry{
			val.ID,
			val.User,
			val.Description,
			val.Project,
			val.Client,
			val.Duration,
			val.Billable,
			val.Start.Unix(),
		}
		entries = append(entries, entry)

	}
	return entries
}

func getTogglReportData(token string, days int) (weekReport toggl.DetailedReport) {
	session := toggl.OpenSession(token)
	account, err := session.GetAccount()
	checkErr(err)
	tNow := time.Now()
	tWeek := tNow.AddDate(0, 0, days)
	weekReport, err = session.GetDetailedReport(
		account.Data.Workspaces[0].ID,
		tWeek.Format("2006-01-02"),
		tNow.Format("2006-01-02"),
		account.Data.ID,
	)
	checkErr(err)
	return weekReport
}
