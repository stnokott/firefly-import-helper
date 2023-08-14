// Package worker includes job handlers for Firefly API and Telegram
package worker

import (
	"firefly-iii-fix-ing/internal/autoimport"
	"firefly-iii-fix-ing/internal/modules"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-co-op/gocron"
)

// Worker handles commands and interference between components*/
type Worker struct {
	fireflyAPI      *fireflyAPI
	telegramBot     *telegramBot
	autoimporter    *autoimport.Manager
	scheduler       *gocron.Scheduler
	healthchecksURL string
	httpClient      *http.Client
}

// AutoimportOptions holds options for the autoimporter*/
type AutoimportOptions struct {
	URL             string
	Port            uint
	Secret          string
	CronSchedule    string
	HealthchecksURL string
}

// TelegramOptions holds options for the telegram worker*/
type TelegramOptions struct {
	AccessToken string
	ChatID      int64
}

const cronTag = "autoimport"

// NewWorker creates a new worker instance*/
func NewWorker(fireflyAccessToken string, fireflyBaseURL string, autoimportOptions *AutoimportOptions, telegramOptions *TelegramOptions) (*Worker, error) {
	// remove trailing slash from Firefly III base URL
	if fireflyBaseURL[len(fireflyBaseURL)-1:] == "/" {
		fireflyBaseURL = fireflyBaseURL[:len(fireflyBaseURL)-1]
	}

	bot, err := NewBot(telegramOptions.AccessToken, telegramOptions.ChatID)
	if err != nil {
		return nil, err
	}

	fireflyAPI := newFireflyAPI(
		fireflyBaseURL,
		fireflyAccessToken,
		modules.NewModuleHandler(),
		bot,
	)
	bot.transactionUpdater = fireflyAPI

	autoimporter, err := autoimport.NewManager(autoimportOptions.URL, autoimportOptions.Port, autoimportOptions.Secret)
	if err != nil {
		return nil, err
	}

	scheduler := gocron.NewScheduler(time.Local)
	log.Printf("Initiated new Cron scheduler with timezone %s", scheduler.Location())

	w := &Worker{
		telegramBot:     bot,
		fireflyAPI:      fireflyAPI,
		autoimporter:    autoimporter,
		scheduler:       scheduler,
		healthchecksURL: autoimportOptions.HealthchecksURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	log.Println("Setting up autoimport...")
	job, err := scheduler.Cron(autoimportOptions.CronSchedule).Tag(cronTag).Do(w.Autoimport)
	if err != nil {
		return nil, err
	} else if job.Error() != nil {
		return nil, err
	}
	log.Printf(">> Autoimport scheduled with cron '%s'", autoimportOptions.CronSchedule)

	return w, nil
}

// Autoimport runs the autoimport, messages healthchecks if needed and changes the config files afterwards*/
func (w *Worker) Autoimport() {
	w.pingHealthchecks(healthchecksStart)
	log.Println("Running autoimport...")

	var err error
	defer func() {
		log.Println(">> Done, next at", w.getNextAutoimportAsString())
		if err != nil {
			w.pingHealthchecks(healthchecksFailed)
			if errInner := w.telegramBot.NotifyError(err); errInner != nil {
				log.Println("error sending notification:", errInner)
				log.Println("initial error:", err)
				return
			}
		} else {
			w.pingHealthchecks(healthchecksSuccess)
		}
	}()

	var filepaths []string
	filepaths, err = w.autoimporter.GetJsonFilePaths()
	if err != nil {
		return
	}
	for _, jsonPath := range filepaths {
		log.Println(">>", filepath.Base(jsonPath))
		if err = w.autoimporter.Import(jsonPath); err != nil {
			log.Println(">> got error:", err)
			err = fmt.Errorf("could not autoimport config %s: %s", filepath.Base(jsonPath), err)
			return
		}
	}
}

type healthchecksType uint8

const (
	healthchecksStart healthchecksType = iota
	healthchecksSuccess
	healthchecksFailed
)

func (w *Worker) pingHealthchecks(typ healthchecksType) {
	if w.healthchecksURL != "" {
		healthchecksURL := w.healthchecksURL
		switch typ {
		case healthchecksStart:
			healthchecksURL += "/start"
		case healthchecksFailed:
			healthchecksURL += "/fail"
		}
		log.Printf("Pinging %s...", healthchecksURL)
		_, err := w.httpClient.Head(healthchecksURL)
		if err != nil {
			log.Println("WARNING: could not ping healthchecks:", err)
		}
	}
}

func (w *Worker) getNextAutoimportAsString() string {
	jobs, err := w.scheduler.FindJobsByTag(cronTag)
	if err != nil {
		return "n/a"
	}

	return jobs[0].NextRun().Format("02.01.2006 15:04:05")
}

// Listen starts webserver and ensures a webhook in Firefly exists, pointing to this server*/
func (w *Worker) Listen() error {
	log.Println("Ensuring webhook exists...")
	url, err := w.fireflyAPI.createOrUpdateWebhook()
	if err != nil {
		return fmt.Errorf(">> error occured: %w", err)
	}
	log.Println(">> Webhook ready at", url)
	log.Println()

	// start telegram bot
	go w.telegramBot.Listen()
	w.scheduler.StartAsync()

	// run immediately if not schedule in next 3 minutes
	if _, nextRun := w.scheduler.NextRun(); time.Until(nextRun).Minutes() >= 3 {
		go func() {
			time.Sleep(10 * time.Second)
			w.Autoimport()
		}()
	}

	log.Println("Next autoimport scheduled for", w.getNextAutoimportAsString())
	log.Println()
	return w.fireflyAPI.Listen()
}
