package chatbot

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/doylecnn/new-nsfc-bot/storage"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	lru "github.com/hashicorp/golang-lru"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	tgbot        *tgbotapi.BotAPI
	botAdminID   int
	_projectID   string
	_domain      string
	cacheForEdit *lru.Cache
	sentMsgs     []sentMessage
)

type sentMessage struct {
	ChatID int64
	MsgID  int
	Time   time.Time
}

// ChatBot is chat bot
type ChatBot struct {
	TgBotClient *tgbotapi.BotAPI
	Route       Router
	ProjectID   string
	appID       string
	token       string
}

// NewChatBot return new chat bot
func NewChatBot(token, domain, appID, projectID, port string, adminID int) ChatBot {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logrus.Fatalln(err)
	}

	tgbot = bot

	var botHelpText = `/addfc 添加你的fc，可批量添加：/addfc id1:fc1;id2:fc2……
/delfc [fc] 用于删除已登记的FC
/myfc 显示自己的所有fc
/sfc 搜索你回复或at 的人的fc
/fc 与sfc 相同
/fclist 列出本群所有人的fc 列表
/whois name 查找NSAccount/Island是 name 的用户
/addisland 添加你的动森岛屿：/addisland 岛名 N/S 岛主 岛屿简介等信息
/islandinfo 更新你的动森岛屿基本信息和简介：/updateBaseInfo 简介
/settimezone 设置岛屿所在的时区，[-12:00, +12:00]
/sac 搜索你回复或at 的人的AnimalCrossing 信息
/myisland 显示自己的岛信息
/open 开放自己的岛 命令后可以附上岛屿今日特色内容
/close 关闭自己的岛
/dtcj 更新大头菜价格, 不带参数时，和 /gj 相同
/weekprice 当周菜价回看/预测
/gj 大头菜最新价格，只显示同群中价格从高到低前10
/ghs 搞化石交换用的登记spreadsheet链接
/islands 提供网页展示本bot 记录的所有动森岛屿信息
/login 登录到本bot 的web 界面，更方便查看信息
/help 查看本帮助信息`
	var botCommands []BotCommand
	scanner := bufio.NewScanner(strings.NewReader(botHelpText))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		sep := strings.SplitN(scanner.Text(), " ", 2)
		botCommands = append(botCommands,
			BotCommand{Command: strings.TrimSpace(sep[0])[1:],
				Description: strings.TrimSpace(sep[1]),
			})
	}
	if cmds, err := getMyCommands(); err != nil {
		logrus.WithError(err).Error("getMyCommands")
	} else if err == nil && len(cmds) != len(botCommands) {
		if resp, err := setMyCommands(botCommands); err != nil {
			logrus.WithError(err).Warn("setMyCommands")
		} else {
			logrus.WithFields(logrus.Fields{"result": resp.Result}).Info("setMyCommands")
		}
	}

	router := NewRouter()
	botAdminID = adminID
	_projectID = projectID
	_domain = domain
	c := ChatBot{TgBotClient: bot, Route: router, ProjectID: projectID, appID: appID, token: token}

	router.HandleFunc("help", func(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: botHelpText}},
			nil
	})

	// 仅占位用
	router.HandleFunc("start", cmdStart)

	router.HandleFunc("addfc", cmdAddFC)
	router.HandleFunc("delfc", cmdDelFC)
	router.HandleFunc("myfc", cmdMyFC)
	router.HandleFunc("sfc", cmdSearchFC)
	router.HandleFunc("fc", cmdSearchFC)
	router.HandleFunc("fclist", cmdListFriendCodes)
	//router.HandleFunc("deleteme", cmdDeleteMe)

	// Animal Crossing: New Horizons
	router.HandleFunc("islands", cmdListIslands)
	router.HandleFunc("addisland", cmdAddMyIsland)
	router.HandleFunc("islandinfo", cmdUpdateIslandBaseInfo)
	router.HandleFunc("updateBaseInfo", cmdUpdateIslandBaseInfo)
	router.HandleFunc("settimezone", cmdSetIslandTimezone)
	router.HandleFunc("myisland", cmdMyIsland)
	router.HandleFunc("open", cmdOpenIsland)
	router.HandleFunc("close", cmdCloseIsland)
	router.HandleFunc("dtcj", cmdDTCPriceUpdate)
	router.HandleFunc("weekprice", cmdDTCWeekPriceAndPredict)
	router.HandleFunc("gj", cmdDTCMaxPriceInGroup)
	router.HandleFunc("sac", cmdSearchAnimalCrossingInfo)
	router.HandleFunc("ghs", cmdHuaShiJiaoHuanBiaoGe)
	router.HandleFunc("whois", cmdWhois)

	// queue
	router.HandleFunc("queue", cmdOpenIslandQueue)
	router.HandleFunc("myqueue", cmdMyQueue)
	//router.HandleFunc("updatepassword", cmdUpdatePassword) 私聊直接回复特定消息触发
	router.HandleFunc("list", cmdJoinedQueue)
	router.HandleFunc("dismiss", cmdDismissIslandQueue)

	// web login
	router.HandleFunc("login", cmdWebLogin)

	// admin
	router.HandleFunc("importDATA", cmdImportData)
	router.HandleFunc("updatetimezone", cmdUpgradeData)
	router.HandleFunc("fclistall", cmdListAllFriendCodes)
	router.HandleFunc("debug", cmdToggleDebugMode)
	router.HandleFunc("clear", c.cmdClearMessages)

	logrus.WithFields(logrus.Fields{"bot username": bot.Self.UserName,
		"bot id": bot.Self.ID}).Infof("Authorized on account %s, ID: %d", bot.Self.UserName, bot.Self.ID)

	if err = c.SetWebhook(); err != nil {
		logrus.Error(err)
	}
	if cacheForEdit, err = lru.New(17); err != nil {
		logrus.WithError(err).Error("new lru cache failed")
	}

	return c
}

// SetWebhook set webhook
func (c ChatBot) SetWebhook() (err error) {
	info, err := c.TgBotClient.GetWebhookInfo()
	if err != nil {
		return
	}
	if info.LastErrorDate != 0 {
		logrus.WithField("last error message", info.LastErrorMessage).Info("Telegram callback failed")
	}
	if !info.IsSet() {
		var webhookConfig WebhookConfig
		var wc = tgbotapi.NewWebhook(fmt.Sprintf("https://%s.appspot.com/%s", c.appID, c.token))
		webhookConfig = WebhookConfig{WebhookConfig: wc}
		webhookConfig.MaxConnections = 10
		webhookConfig.AllowedUpdates = []string{"message", "edited_message", "inline_query", "callback_query"}
		_, err = c.setWebhook(webhookConfig)
		if err != nil {
			logrus.WithError(err).Error("SetWebhook failed")
			return
		}
	}
	return
}

// MessageHandler process message
func (c ChatBot) MessageHandler(updates chan tgbotapi.Update) {
	for i := 0; i < 2; i++ {
		go c.messageHandlerWorker(updates)
	}
}

func (c ChatBot) messageHandlerWorker(updates chan tgbotapi.Update) {
	for update := range updates {
		inlineQuery := update.InlineQuery
		callbackQuery := update.CallbackQuery
		message := update.Message
		var isEditedMessage bool = false
		if message == nil {
			message = update.EditedMessage
			if message != nil {
				isEditedMessage = true
			}
		}
		if inlineQuery != nil {
			c.HandleInlineQuery(inlineQuery)
		} else if callbackQuery != nil {
			c.HandleCallbackQuery(callbackQuery)
		} else if message != nil && message.Chat.IsPrivate() && !message.IsCommand() && message.ReplyToMessage != nil && message.ReplyToMessage.Text == "请输入新的密码" && message.ReplyToMessage.From.IsBot {
			replies, e := cmdUpdatePassword(message)
			if e != nil {
				logrus.WithError(e).Error("cmdUpdatePassword")
			}
			for _, reply := range replies {
				_, e := c.TgBotClient.Send(reply)
				if e != nil {
					logrus.WithError(e).Error("cmdUpdatePassword send message")
				}
			}
		} else if message != nil && message.IsCommand() {
			if message.Chat.IsGroup() || message.Chat.IsSuperGroup() || message.Chat.IsPrivate() {
				if len(sentMsgs) > 0 {
					sort.Slice(sentMsgs, func(i, j int) bool {
						return sentMsgs[i].Time.After(sentMsgs[j].Time)
					})
					var i = 0
					var foundOutDateMsg = false
					for j, sentMsg := range sentMsgs {
						if time.Since(sentMsg.Time).Minutes() > 2 {
							foundOutDateMsg = true
							i = j
							break
						}
					}
					if foundOutDateMsg {
						logrus.WithField("sentMsgs len:", len(sentMsgs))
						for _, sentMsg := range sentMsgs[i:] {
							c.TgBotClient.DeleteMessage(tgbotapi.NewDeleteMessage(sentMsg.ChatID, sentMsg.MsgID))
						}
						copy(sentMsgs, sentMsgs[i:])
						sentMsgs = sentMsgs[:len(sentMsgs)-i]
						logrus.WithField("sentMsgs len:", len(sentMsgs))
					}
				}
				messageSendTime := message.Time()
				if !isEditedMessage && time.Since(messageSendTime).Seconds() > 30 {
					logrus.Infof("old message dropped: %s", message.Text)
					continue
				}
				if isEditedMessage {
					if time.Since(messageSendTime).Minutes() > 2 {
						logrus.Infof("old message dropped: %s", message.Text)
						continue
					}
					var editTime = time.Unix(int64(message.EditDate), 0)
					if time.Since(editTime).Seconds() > 30 {
						logrus.Infof("old message dropped: %s", message.Text)
						continue
					}
				}
				var key string = fmt.Sprintf("%d:%d:%d", message.Chat.ID, message.From.ID, messageSendTime.Unix())
				var canEditSentMsg = false
				var canEditSentMsgID int
				if isEditedMessage {
					logrus.WithField("editedmessage", message.Text).Info("editedmessage received")
					if cacheForEdit != nil {
						if v, ok := cacheForEdit.Get(key); ok {
							if ids, ok := v.([]int); ok {
								if len(ids) > 1 {
									for _, id := range ids {
										c.TgBotClient.DeleteMessage(tgbotapi.NewDeleteMessage(message.Chat.ID, id))
									}
									cacheForEdit.Remove(key)
								} else {
									canEditSentMsg = true
									canEditSentMsgID = ids[0]
								}
							}
						}
					}
				}
				c.TgBotClient.Send(tgbotapi.NewChatAction(message.Chat.ID, tgbotapi.ChatTyping))
				replyMessages, err := c.Route.Run(message)
				var sentMessageIDs []int
				if err != nil {
					if status.Code(err.InnerError) != codes.NotFound {
						logrus.Warnf("%s", err.InnerError)
					}
					if len(err.ReplyText) > 0 {
						replyMessage := tgbotapi.MessageConfig{
							BaseChat: tgbotapi.BaseChat{
								ChatID:           message.Chat.ID,
								ReplyToMessageID: message.MessageID},
							Text: err.ReplyText}
						sentM, err := c.TgBotClient.Send(replyMessage)
						if err != nil {
							logrus.WithError(err).Error("send message failed")
						} else {
							if cacheForEdit != nil {
								sentMessageIDs = append(sentMessageIDs, sentM.MessageID)
								cacheForEdit.Add(key, sentMessageIDs)
							}
							if !message.Chat.IsPrivate() && message.Command() != "open" {
								sentMsgs = append(sentMsgs, sentMessage{ChatID: message.Chat.ID, MsgID: sentM.MessageID, Time: sentM.Time()})
							}
						}
					}
				} else {
					if canEditSentMsg && len(replyMessages) == 1 && replyMessages[0] != nil {
						if l := len(replyMessages[0].Text); l > 0 {
							fm := tgbotapi.EditMessageTextConfig{
								BaseEdit: tgbotapi.BaseEdit{
									ChatID:    message.Chat.ID,
									MessageID: canEditSentMsgID},
								Text: replyMessages[0].Text}
							_, err := c.TgBotClient.Send(fm)
							if err != nil {
								logrus.WithError(err).Error("send message failed")
							}
						}
					} else {
						for _, replyMessage := range replyMessages {
							var text = replyMessage.Text
							l := len(text)
							if replyMessage != nil && l > 0 {
								if l > 4096 {
									offset := 0
									for l > 4096 {
										remain := l - offset
										if remain > 4096 {
											remain = 4096
										}
										fm := tgbotapi.MessageConfig{
											BaseChat: tgbotapi.BaseChat{
												ChatID:           replyMessage.ChatID,
												ReplyToMessageID: message.MessageID},
											Text: replyMessage.Text[offset : offset+remain]}
										sentM, err := c.TgBotClient.Send(fm)
										if err != nil {
											logrus.WithError(err).Error("send message failed")
										} else if replyMessage.ChatID == message.Chat.ID && !message.Chat.IsPrivate() {
											if cacheForEdit != nil {
												sentMessageIDs = append(sentMessageIDs, sentM.MessageID)
											}
											if !message.Chat.IsPrivate() && message.Command() != "open" {
												sentMsgs = append(sentMsgs, sentMessage{ChatID: message.Chat.ID, MsgID: sentM.MessageID, Time: sentM.Time()})
											}
										}
									}
								} else {
									sentM, err := c.TgBotClient.Send(*replyMessage)
									if err != nil {
										logrus.WithError(err).Error("send message failed")
									} else if replyMessage.ChatID == message.Chat.ID && !message.Chat.IsPrivate() {
										if cacheForEdit != nil {
											sentMessageIDs = append(sentMessageIDs, sentM.MessageID)
										}
										if message.Command() != "open" {
											sentMsgs = append(sentMsgs, sentMessage{ChatID: message.Chat.ID, MsgID: sentM.MessageID, Time: sentM.Time()})
										}
									}
								}
								if len(sentMessageIDs) > 0 && cacheForEdit != nil {
									cacheForEdit.Add(key, sentMessageIDs)
								}
							}
						}
					}
				}
			} else if message != nil && message.LeftChatMember != nil {
				if message.Chat.IsPrivate() {
					continue
				}
				logrus.WithFields(logrus.Fields{
					"uid":   message.LeftChatMember.ID,
					"name":  message.LeftChatMember.FirstName + " " + message.LeftChatMember.LastName + "(" + message.LeftChatMember.UserName + ")",
					"gid":   message.Chat.ID,
					"group": message.Chat.Title,
				}).Info("user left group")
				if message.Chat.IsPrivate() {
					continue
				}
				var groupID int64 = message.Chat.ID
				ctx := context.Background()
				if err := storage.RemoveGroupIDFromUserGroupIDs(ctx, message.From.ID, groupID); err != nil {
					logrus.WithError(err).Error("remove groupid from user's groupids failed")
				}
			} else if message != nil && message.NewChatMembers != nil && len(*message.NewChatMembers) > 0 {
				if message.Chat.IsPrivate() {
					continue
				}
				ctx := context.Background()
				g := storage.Group{ID: message.Chat.ID, Type: message.Chat.Type, Title: message.Chat.Title}
				og, err := storage.GetGroup(ctx, g.ID)
				if err != nil {
					if status.Code(err) == codes.NotFound {
						g.Set(ctx)
					} else {
						logrus.WithError(err).Error("get group failed")
					}
				} else {
					if og.Title != g.Title || og.Type != g.Type {
						g.Update(ctx)
					}
				}
				for _, u := range *message.NewChatMembers {
					logrus.WithFields(logrus.Fields{
						"uid":   u.ID,
						"name":  u.FirstName + " " + u.LastName + "(" + u.UserName + ")",
						"gid":   message.Chat.ID,
						"group": message.Chat.Title,
					}).Info("user joined group")
					u, err := storage.GetUser(ctx, u.ID, g.ID)
					if err != nil {
						logrus.WithError(err).Error("get user failed")
					} else {
						if len(u.GroupIDs) > 0 {
							u.GroupIDs = append(u.GroupIDs, g.ID)
						} else {
							u.GroupIDs = []int64{g.ID}
						}
						if err = storage.AddGroupIDToUserGroupIDs(ctx, u.ID, g.ID); err != nil {
							logrus.WithError(err).Error("add groupid to user's groupids faild")
						}
					}
				}
			}
		}
	}
}

// WebhookConfig contains information about a SetWebhook request.
type WebhookConfig struct {
	tgbotapi.WebhookConfig
	AllowedUpdates []string
}

// SetWebhook sets a webhook.
//
// If this is set, GetUpdates will not get any data!
//
// If you do not have a legitimate TLS certificate, you need to include
// your self signed certificate with the config.
func (c ChatBot) setWebhook(config WebhookConfig) (tgbotapi.APIResponse, error) {
	if config.Certificate == nil {
		v := url.Values{}
		v.Add("url", config.URL.String())
		if config.MaxConnections != 0 {
			v.Add("max_connections", strconv.Itoa(config.MaxConnections))
		}
		if len(config.AllowedUpdates) != 0 {
			v["allowed_updates"] = config.AllowedUpdates
		}

		return c.TgBotClient.MakeRequest("setWebhook", v)
	}

	params := make(map[string]string)
	params["url"] = config.URL.String()
	if config.MaxConnections != 0 {
		params["max_connections"] = strconv.Itoa(config.MaxConnections)
	}

	resp, err := c.TgBotClient.UploadFile("setWebhook", params, "certificate", config.Certificate)
	if err != nil {
		return tgbotapi.APIResponse{}, err
	}

	return resp, nil
}

func (c ChatBot) deleteWebhook() (tgbotapi.APIResponse, error) {
	return c.TgBotClient.MakeRequest("deleteWebhook", url.Values{})
}

// RestartBot restart the bot
func (c ChatBot) RestartBot() {
	info, err := c.TgBotClient.GetWebhookInfo()
	if err != nil {
		return
	}
	if info.LastErrorDate != 0 {
		logrus.WithField("last error message", info.LastErrorMessage).Info("Telegram callback failed")
	}
	if info.IsSet() {
		var resp tgbotapi.APIResponse
		if resp, err = c.deleteWebhook(); err != nil {
			logrus.WithError(err).Error(resp)
			return
		}
	}
	c.SetWebhook()
}

func markdownSafe(text string) string {
	var escapeCharacters = []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, c := range escapeCharacters {
		text = strings.ReplaceAll(text, c, fmt.Sprintf("\\%s", c))
	}
	return text
}
