// Package telegram handles the Telegram bot interactions.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	telebot "github.com/go-telegram/bot"
	telemodels "github.com/go-telegram/bot/models"
)

// Bot allows for high-level interactions with the Telegram Bot API.
type Bot interface {
	// Run starts listening to message updates, tied to the provided context.
	Run(ctx context.Context) error
}

type bot struct {
	*telebot.Bot
	chatID string
}

const callbackDataPrefix = "category:"

func NewBot(token string, chatID string) (Bot, error) {
	b, err := telebot.New(
		token,
		telebot.WithCallbackQueryDataHandler(callbackDataPrefix, telebot.MatchTypePrefix, callbackQueryDataHandler),
		telebot.WithErrorsHandler(func(err error) {
			slog.Error(err.Error())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create telegram bot: %w", err)
	}
	return &bot{
		Bot:    b,
		chatID: chatID,
	}, nil
}

func (b *bot) Run(ctx context.Context) error {
	_, err := b.SendMessage(ctx, &telebot.SendMessageParams{
		ChatID:         b.chatID,
		ParseMode:      telemodels.ParseModeMarkdown,
		ProtectContent: true,
		Text: `
			*Hello\!*
			Test message from Bot v2\!
		`,
		ReplyMarkup: buildInlineKeyboard([]string{"A", "B", "C"}, ""),
	})
	if err != nil {
		return err
	}
	slog.Info("ready to receive messages")
	b.Start(ctx)
	return nil
}

func callbackQueryDataHandler(ctx context.Context, bot *telebot.Bot, update *telemodels.Update) {
	// answer callback from button press in chat
	_, err := bot.AnswerCallbackQuery(ctx, &telebot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "Pressed button with data '" + update.CallbackQuery.Data + "'",
	})
	if err != nil {
		slog.Error("failed to answer callback query: " + err.Error())
	}

	// edit keyboard to indicate changes
	attachedMessage := update.CallbackQuery.Message
	if attachedMessage.Message == nil {
		slog.Error("received callback query without message data, ignoring")
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
		slog.Error("failed to edit message after callback: " + err.Error())
	}
}

func buildInlineKeyboard(options []string, selectedOption string) telemodels.InlineKeyboardMarkup {
	cols := 3
	var buttons [][]telemodels.InlineKeyboardButton
	for i := 0; i < len(options); i += cols {
		end := min(i+cols, len(options))
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
