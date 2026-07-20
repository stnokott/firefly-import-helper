// Package telegram handles the Telegram bot interactions.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	telebot "github.com/go-telegram/bot"
	telemodels "github.com/go-telegram/bot/models"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("telegram")

type bot struct {
	t              *telebot.Bot
	chatID         string
	firefly        domain.FireflyReadWriter
	fireflyBaseURL url.URL
}

var _ domain.Messenger = (*bot)(nil)

const callbackPrefixCategory = "category:"

func NewBot(token string, fireflyBaseURL url.URL, telegramChatID string, ffAPI domain.FireflyReadWriter) (domain.Messenger, error) {
	b := &bot{
		chatID:         telegramChatID,
		firefly:        ffAPI,
		fireflyBaseURL: fireflyBaseURL,
	}

	t, err := telebot.New(
		token,
		telebot.WithCallbackQueryDataHandler(callbackPrefixCategory, telebot.MatchTypePrefix, b.callbackHandlerCategory),
		telebot.WithErrorsHandler(func(err error) {
			logger.Errorv(err)
		}),
		telebot.WithWorkers(1),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create telegram bot: %w", err)
	}

	b.t = t
	return b, nil
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

func (b *bot) MsgNewTransaction(ctx context.Context, data *domain.TransactionRead, ffCategories []string) error {
	msg, err := renderNewTransaction(data)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return b.send(ctx, msg, withCategoriesKeyboard(data.FireflyID, ffCategories, data.Category))
}

func (b *bot) send(ctx context.Context, html string, opts ...sendOption) error {
	params := &telebot.SendRichMessageParams{
		ChatID: b.chatID,
		RichMessage: telemodels.InputRichMessage{
			HTML: html,
		},
	}
	for _, opt := range opts {
		opt(params)
	}

	_, err := b.t.SendRichMessage(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

type sendOption func(params *telebot.SendRichMessageParams)

func withCategoriesKeyboard(transactionID string, categories []string, selectedCategory string) sendOption {
	return func(params *telebot.SendRichMessageParams) {
		inlineMarkup := buildInlineCategoryKeyboard(transactionID, categories, selectedCategory)
		params.ReplyMarkup = inlineMarkup
	}
}

func encodeCategoryCallbackData(transactionID, category string) string {
	return callbackPrefixCategory + transactionID + ":" + category
}

func decodeCategoryCallbackData(data string) (transactionID, category string) {
	parts := strings.Split(data, ":")
	return parts[1], parts[2]
}

const (
	buttonsPerRow       = 3
	buttonStyleSelected = "success"
)

func buildInlineCategoryKeyboard(transactionID string, categories []string, selectedCategory string) telemodels.InlineKeyboardMarkup {
	var buttons [][]telemodels.InlineKeyboardButton
	for i := 0; i < len(categories); i += buttonsPerRow {
		end := min(i+buttonsPerRow, len(categories))
		var row []telemodels.InlineKeyboardButton
		for _, category := range categories[i:end] {
			button := telemodels.InlineKeyboardButton{
				Text:         category,
				CallbackData: encodeCategoryCallbackData(transactionID, category),
			}
			if category == selectedCategory {
				button.Style = buttonStyleSelected
			}
			row = append(row, button)
		}
		buttons = append(buttons, row)
	}

	return telemodels.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func getSelectedKeyboardButtonData(buttons [][]telemodels.InlineKeyboardButton) string {
	for _, row := range buttons {
		for i := range row {
			if row[i].Style == "success" {
				return row[i].CallbackData
			}
		}
	}
	return ""
}

func (b *bot) callbackHandlerCategory(ctx context.Context, bot *telebot.Bot, update *telemodels.Update) {
	transactionID, selectedCategory := decodeCategoryCallbackData(update.CallbackQuery.Data)

	var err error
	defer func() {
		var callbackText string
		if err != nil {
			callbackText = "ERROR: " + err.Error()
			logger.Errorv(err)
		} else {
			callbackText = fmt.Sprintf("Category for #%s set to %q", transactionID, selectedCategory)
			logger.Info(callbackText)
		}

		// trim to max length of 200 chars
		if utf8.RuneCountInString(callbackText) > 200 {
			callbackText = callbackText[:197] + "..."
		}
		_, innerErr := bot.AnswerCallbackQuery(ctx, &telebot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            callbackText,
		})
		if innerErr != nil {
			logger.Errorv(innerErr)
		}
	}()

	if err = validateSelectedCategory(update.CallbackQuery); err != nil {
		return
	}

	attachedMessage := update.CallbackQuery.Message
	if attachedMessage.Message == nil {
		err = errors.New("received callback query without message data, ignoring")
		return
	}

	var updated *domain.TransactionRead
	updated, err = b.firefly.UpdateTransactionCategory(ctx, transactionID, selectedCategory)
	if err != nil {
		return
	}

	// update keyboard to reflect new selected category
	var categories []string
	if categories, err = b.firefly.ListCategories(ctx); err != nil {
		return
	}
	replyMarkup := buildInlineCategoryKeyboard(transactionID, categories, updated.Category)

	_, err = bot.EditMessageReplyMarkup(ctx, &telebot.EditMessageReplyMarkupParams{
		ChatID:          attachedMessage.Message.Chat.ID,
		MessageID:       attachedMessage.Message.ID,
		InlineMessageID: update.CallbackQuery.InlineMessageID,
		ReplyMarkup:     replyMarkup,
	})
	if err != nil {
		err = fmt.Errorf("failed to update inline keyboard: %w", err)
	}
}

func validateSelectedCategory(callback *telemodels.CallbackQuery) error {
	previouslySelectedCategoryData := getSelectedKeyboardButtonData(callback.Message.Message.ReplyMarkup.InlineKeyboard)
	if previouslySelectedCategoryData == "" {
		return nil
	}
	_, previouslySelectedCategory := decodeCategoryCallbackData(previouslySelectedCategoryData)
	_, selectedCategory := decodeCategoryCallbackData(callback.Data)
	if previouslySelectedCategory != "" && selectedCategory == previouslySelectedCategory {
		return fmt.Errorf("transaction already has category %q", selectedCategory)
	}
	return nil
}
