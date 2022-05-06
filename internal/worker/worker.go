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

type Worker struct {
	fireflyApi      *fireflyApi
	telegramBot     *telegramBot
	autoimporter    *autoimport.Manager
	scheduler       *gocron.Scheduler
	healthchecksUrl string
	httpClient      *http.Client
}

type AutoimportOptions struct {
	Url             string
	Port            uint
	Secret          string
	CronSchedule    string
	HealthchecksUrl string
}

type TelegramOptions struct {
	AccessToken string
	ChatId      int64
}

const cronTag = "autoimport"

func NewWorker(fireflyAccessToken string, fireflyBaseUrl string, autoimportOptions *AutoimportOptions, telegramOptions *TelegramOptions) (*Worker, error) {
	// remove trailing slash from Firefly III base URL
	if fireflyBaseUrl[len(fireflyBaseUrl)-1:] == "/" {
		fireflyBaseUrl = fireflyBaseUrl[:len(fireflyBaseUrl)-1]
	}

	bot, err := NewBot(telegramOptions.AccessToken, telegramOptions.ChatId)
	if err != nil {
		return nil, err
	}

	fireflyApi := newFireflyApi(
		fireflyBaseUrl,
		fireflyAccessToken,
		modules.NewModuleHandler(),
		bot,
	)
	bot.transactionUpdater = fireflyApi

	autoimporter, err := autoimport.NewManager(autoimportOptions.Url, autoimportOptions.Port, autoimportOptions.Secret)
	if err != nil {
		return nil, err
	}

	scheduler := gocron.NewScheduler(time.Local)

	w := &Worker{
		telegramBot:     bot,
		fireflyApi:      fireflyApi,
		autoimporter:    autoimporter,
		scheduler:       scheduler,
		healthchecksUrl: autoimportOptions.HealthchecksUrl,
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

func (w *Worker) Autoimport() {
	log.Println("Running autoimport...")
	if w.healthchecksUrl != "" {
		w.pingHealthchecks(healthchecksStart)
	}

	filepaths, err := w.autoimporter.GetJsonFilePaths()
	if err != nil {
		if errInner := w.telegramBot.NotifyError(err); errInner != nil {
			log.Println("error sending notification:", errInner)
			log.Println("initial error:", err)
			w.pingHealthchecks(healthchecksFailed)
			return
		}
	}
	for _, jsonPath := range filepaths {
		log.Println(">>", filepath.Base(jsonPath))
		if err := w.autoimporter.Import(jsonPath); err != nil {
			log.Println(">> got error:", err)
			err := fmt.Errorf("could not autoimport config %s: %s", filepath.Base(jsonPath), err)
			if errInner := w.telegramBot.NotifyError(err); errInner != nil {
				log.Println("error sending notification:", errInner)
				log.Println("initial error:", err)
				w.pingHealthchecks(healthchecksFailed)
			}
		}
	}
	w.pingHealthchecks(healthchecksSuccess)
	log.Println(">> Done, next at", w.getNextAutoimportAsString())
}

type healthchecksType uint8

const (
	healthchecksStart healthchecksType = iota
	healthchecksSuccess
	healthchecksFailed
)

func (w *Worker) pingHealthchecks(type_ healthchecksType) {
	healthchecksUrl := w.healthchecksUrl
	switch type_ {
	case healthchecksStart:
		healthchecksUrl += "/start"
	case healthchecksFailed:
		healthchecksUrl += "/fail"
	}
	fmt.Printf("Pinging %s...", healthchecksUrl)
	_, err := w.httpClient.Head(healthchecksUrl)
	if err != nil {
		log.Println("WARNING: could not ping healthchecks:", err)
	}
}

func (w *Worker) getNextAutoimportAsString() string {
	jobs, err := w.scheduler.FindJobsByTag(cronTag)
	if err != nil {
		return "n/a"
	} else {
		return jobs[0].NextRun().Format("02.01.2006 15:04:05")
	}
}

func (w *Worker) Listen() error {
	log.Println("Ensuring webhook exists...")
	url, err := w.fireflyApi.createOrUpdateWebhook()
	if err != nil {
		return err
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
	return w.fireflyApi.Listen()
}
