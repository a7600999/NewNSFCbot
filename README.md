# NS_FC_bot
----

### 在几个好友 tg 群中使用tg 机器人。
### 部署在GAE 上。本地部署的老版本在[此](https://github.com/doylecnn/NS_FC_bot)
### 使用Cloud Firestore 存储数据。

### 支持的命令
以下列出的命令，除非特别标注，均可私聊bot 操作

#### 用于等级Nintendo Switch 账户的 Friend Code

- /addfc 添加你的fc，可批量添加：/addfc id1:fc1;id2:fc2……
- /delfc [fc] 用于删除已登记的FC
- /myfc 显示自己的所有fc
- /sfc 搜索你回复或at 的人的fc *只能群聊使用*
- /fclist 列出本群所有人的fc 列表 *只能群聊使用*

#### 同时提供一些动森岛屿相关的功能

- /whois name 查找NSAccount/Island是 name 的用户 *只能群聊使用*
- /addisland 添加你的动森岛：/addisland 岛名 N/S 岛主 其它信息
- /islandinfo 更新你的动森岛屿基本信息和简介
- /settimezone 设置岛屿所在的时区，[-12:00, +12:00]
- /sac 搜索你回复或at 的人的AnimalCrossing 信息 *只能群聊使用*
- /myisland 显示自己的岛信息
- /open 开放自己的岛 命令后可以附上岛屿今日特色内容
- /close 关闭自己的岛
- /dtcj 更新大头菜价格, 不带参数时，和 /gj 相同
- /weekprice 当周菜价回看/预测
- /gj 大头菜最新价格，只显示同群中价格从高到低前5，周日则相反 *只能群聊使用*
- /islands 提供网页展示本bot 记录的所有动森岛屿信息
- /login 登录到本bot 的web 界面，更方便查看信息

#### 用于动森岛屿上岛排队的功能
用于排队的功能大部分都只能私聊进行，使用 inlineKeyboard 完成功能

队列主：
- /queue [密码] 开启新的队列
- /queue [密码] [开岛说明] 开启新的队列，同时更新开岛说明
- /queue [密码] [开岛说明] [最大客人数] 开启新的队列，同时更新开岛说明，同时根据队列信息，半自动邀请下一位旅客（尚未实现）
- /myqueue 列出自己创建的队列
- /dismiss 解散自己创建的队列

队列参与者：
- /list 列出自己加入的队列


#### 使用help 命令查看帮助信息
- /help 查看本帮助信息

#### 开发中的功能

- [x] 周日报价要看最低的
- [ ] 岛主能查看当前在岛上的都是谁
- [ ] 被岛主主动分享到了哪些群，这些群的群成员才有资格搜索到队列入口/参与排队
- [ ] 炸岛了/岛主更新密码后，当前在岛上的人自动收到新密码（？）
- [ ] 岛主能从队列中选择下一个人是谁（？）
- [ ] 岛主能踢掉队列中的特定的人（？）
- [ ] 排队的人不能查看队列中都有谁
- [ ] 排队的人，在即将轮到自己时（前面还有2人，1人）都收到提醒通知
- [ ] 排队的人倒计时内不答复，视为放弃登岛（那么就不能在通知的时候直接给密码）
- [ ] 队列归零一段时间后，自动解散