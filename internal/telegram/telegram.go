// Package telegram handles the Telegram bot interactions.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	telebot "github.com/go-telegram/bot"
	telemodels "github.com/go-telegram/bot/models"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/server"
)

var logger = log.For("telegram")

// Bot allows for high-level interactions with the Telegram Bot API.
type Bot struct {
	t              *telebot.Bot
	fireflyBaseURL string
	chatID         string
}

const callbackDataPrefix = "category:"

func NewBot(token string, chatID string, fireflyBaseURL string) (*Bot, error) {
	t, err := telebot.New(
		token,
		telebot.WithCallbackQueryDataHandler(callbackDataPrefix, telebot.MatchTypePrefix, callbackQueryDataHandler),
		telebot.WithErrorsHandler(func(err error) {
			logger.ErrorV(err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create telegram bot: %w", err)
	}
	return &Bot{
		t:              t,
		fireflyBaseURL: fireflyBaseURL,
		chatID:         chatID,
	}, nil
}

// Run starts listening to message updates, tied to the provided context.
func (b *Bot) Run(ctx context.Context, dataChan <-chan *server.WebhookResponse) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_, err := b.t.SendMessage(ctx, &telebot.SendMessageParams{
			ChatID: b.chatID,
			Text:   "Firefly-III Import Helper started.",
		})
		if err != nil {
			logger.ErrorV(fmt.Errorf("failed to send startup message: %w", err))
		}
		b.t.Start(ctx)
	})
	wg.Go(func() {
		b.dataListener(ctx, dataChan)
	})

	logger.Info("ready to receive messages")
	wg.Wait()
}

func (b *Bot) dataListener(ctx context.Context, dataChan <-chan *server.WebhookResponse) {
	for {
		select {
		case <-ctx.Done():
			logger.Debug("context closed, stopping listener")
			return
		case t, ok := <-dataChan:
			if !ok {
				logger.Debug("channel closed, stopping listener")
				return
			}
			err := b.sendTransactionMessage(ctx, t)
			if err != nil {
				logger.ErrorV(err)
			}
		}
	}
}

func (b *Bot) sendTransactionMessage(ctx context.Context, t *server.WebhookResponse) error {
	msg, err := b.renderTemplate(t)
	if err != nil {
		return err
	}
	_, err = b.t.SendMessage(ctx, &telebot.SendMessageParams{
		ChatID:         b.chatID,
		ParseMode:      templateParseMode,
		ProtectContent: true,
		Text:           msg,
		// ReplyMarkup: buildInlineKeyboard([]string{"A", "B", "C"}, ""),
	})
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}
	return nil
}

func callbackQueryDataHandler(ctx context.Context, bot *telebot.Bot, update *telemodels.Update) {
	// answer callback from button press in chat
	_, err := bot.AnswerCallbackQuery(ctx, &telebot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "Pressed button with data '" + update.CallbackQuery.Data + "'",
	})
	if err != nil {
		logger.Error("failed to answer callback query: " + err.Error())
	}

	// TODO: API call

	// edit keyboard to indicate changes
	attachedMessage := update.CallbackQuery.Message
	if attachedMessage.Message == nil {
		logger.Error("received callback query without message data, ignoring")
		return
	}
	selectedCategory := strings.TrimPrefix(update.CallbackQuery.Data, callbackDataPrefix)
	_, err = bot.EditMessageReplyMarkup(ctx, &telebot.EditMessageReplyMarkupParams{
		ChatID:          attachedMessage.Message.Chat.ID,
		MessageID:       attachedMessage.Message.ID,
		InlineMessageID: update.CallbackQuery.InlineMessageID,
		ReplyMarkup:     buildInlineKeyboard([]string{"A", "B", "C"}, selectedCategory),
	})
	if err != nil {
		logger.Error("failed to edit message after callback: " + err.Error())
	}
}

const buttonsPerRow = 3

func buildInlineKeyboard(options []string, selectedOption string) telemodels.InlineKeyboardMarkup {
	var buttons [][]telemodels.InlineKeyboardButton
	for i := 0; i < len(options); i += buttonsPerRow {
		end := min(i+buttonsPerRow, len(options))
		var row []telemodels.InlineKeyboardButton
		for _, option := range options[i:end] {
			buttonText := option
			if option == selectedOption {
				buttonText = "<" + buttonText + ">"
			}
			row = append(row, telemodels.InlineKeyboardButton{
				Text: buttonText, CallbackData: callbackDataPrefix + option,
			})
		}
		buttons = append(buttons, row)
	}

	return telemodels.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
