// Package telegram handles the Telegram bot interactions.
package telegram

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	telebot "github.com/go-telegram/bot"
	telemodels "github.com/go-telegram/bot/models"
	"github.com/stnokott/firefly-import-helper/internal/importer"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("telegram")

type Bot struct {
	t              *telebot.Bot
	fireflyBaseURL url.URL
	chatID         string
}

var _ importer.Messenger = (*Bot)(nil)

const callbackDataPrefix = "category:"

func NewBot(token string, fireflyBaseURL url.URL, telegramChatID string) (*Bot, error) {
	t, err := telebot.New(
		token,
		telebot.WithCallbackQueryDataHandler(callbackDataPrefix, telebot.MatchTypePrefix, callbackHandlerCategory),
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
		chatID:         telegramChatID,
	}, nil
}

func (b *Bot) Run(ctx context.Context) {
	_, err := b.t.SendMessage(ctx, &telebot.SendMessageParams{
		ChatID: b.chatID,
		Text:   "Firefly-III Import Helper started.",
	})
	if err != nil {
		logger.ErrorV(fmt.Errorf("failed to send startup message: %w", err))
	}
	// intercept cancel of parent context and send stopped-message. Only then cancel actual context.
	ctxBot, cancelBot := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		_, _ = b.t.SendMessage(ctxBot, &telebot.SendMessageParams{
			ChatID: b.chatID,
			Text:   "Firefly-III Import Helper shutting down.",
		})
		cancelBot()
	}()
	b.t.Start(ctxBot)
}

func (b *Bot) MsgImportStarted(ctx context.Context) error {
	_, err := b.t.SendMessage(ctx, &telebot.SendMessageParams{
		ChatID: b.chatID,
		Text:   "Import starting...",
	})
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}
	return nil
}

func (b *Bot) MsgImportFinished(ctx context.Context, sum []importer.Summary) error {
	msg, err := b.renderTmplSummary(sum)
	if err != nil {
		return err
	}
	_, err = b.t.SendMessage(ctx, &telebot.SendMessageParams{
		ChatID:    b.chatID,
		Text:      msg,
		ParseMode: telemodels.ParseModeHTML,
	})
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}
	return nil
}

func callbackHandlerCategory(ctx context.Context, bot *telebot.Bot, update *telemodels.Update) {
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
