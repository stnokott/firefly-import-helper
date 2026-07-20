// Package telegram handles the Telegram bot interactions.
package telegram

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	telebot "github.com/go-telegram/bot"
	telemodels "github.com/go-telegram/bot/models"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("telegram")

type bot struct {
	t              *telebot.Bot
	fireflyBaseURL url.URL
	chatID         string
}

var _ domain.Messenger = (*bot)(nil)

const callbackDataPrefix = "category:"

func NewBot(token string, fireflyBaseURL url.URL, telegramChatID string) (domain.Messenger, error) {
	t, err := telebot.New(
		token,
		telebot.WithCallbackQueryDataHandler(callbackDataPrefix, telebot.MatchTypePrefix, callbackHandlerCategory),
		telebot.WithErrorsHandler(func(err error) {
			logger.Errorv(err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create telegram bot: %w", err)
	}

	return &bot{
		t:              t,
		fireflyBaseURL: fireflyBaseURL,
		chatID:         telegramChatID,
	}, nil
}

func (b *bot) Listen(ctx context.Context) {
	if err := b.send(ctx, "Firefly-III Import Helper started."); err != nil {
		logger.Errorv(err)
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

func (b *bot) MsgAccountProblem(ctx context.Context, problem domain.AccountProblem) error {
	msg, err := renderAccountProblem(problem)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return b.send(ctx, msg)
}

func (b *bot) MsgImportStarted(ctx context.Context) error {
	return b.send(ctx, "Import starting...")
}

func (b *bot) MsgNewTransaction(ctx context.Context, data *domain.TransactionCreated) error {
	msg, err := renderNewTransaction(data)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return b.send(ctx, msg)
}

func (b *bot) send(ctx context.Context, msg string) error {
	_, err := b.t.SendMessage(ctx, &telebot.SendMessageParams{
		ChatID:    b.chatID,
		Text:      msg,
		ParseMode: telemodels.ParseModeHTML,
	})
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
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
