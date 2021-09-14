package models

import (
	"errors"
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CodeSignal struct {
	Command []string
	Admin   bool
	Handle  func(sender *Sender) interface{}
}

type Sender struct {
	UserID            int
	ChatID            int
	Type              string
	Contents          []string
	MessageID         int
	Username          string
	IsAdmin           bool
	ReplySenderUserID int
}

func (sender *Sender) Reply(msg string) {
	switch sender.Type {
	case "tg":
		SendTgMsg(sender.UserID, msg)
	case "tgg":
		SendTggMsg(sender.ChatID, sender.UserID, msg, sender.MessageID, sender.Username)
	case "qq":
		SendQQ(int64(sender.UserID), msg)
	case "qqg":
		SendQQGroup(int64(sender.ChatID), int64(sender.UserID), msg)
	}
}

func (sender *Sender) JoinContens() string {
	return strings.Join(sender.Contents, " ")
}

func (sender *Sender) IsQQ() bool {
	return strings.Contains(sender.Type, "qq")
}

func (sender *Sender) IsTG() bool {
	return strings.Contains(sender.Type, "tg")
}

func (sender *Sender) handleJdCookies(handle func(ck *JdCookie)) error {
	cks := GetJdCookies()
	a := sender.JoinContens()
	ok := false
	if !sender.IsAdmin || a == "" {
		for i := range cks {
			if strings.Contains(sender.Type, "qq") {
				if cks[i].QQ == sender.UserID {
					if !ok {
						ok = true
					}
					handle(&cks[i])
				}
			} else if strings.Contains(sender.Type, "tg") {
				if cks[i].Telegram == sender.UserID {
					if !ok {
						ok = true
					}
					handle(&cks[i])
				}
			}
		}
		if !ok {
			sender.Reply("你尚未绑定🐶东账号，请发送帮助，根据教程手动抓Cookie后发送到此绑定。")
			return errors.New("你尚未绑定🐶东账号，请发送帮助，根据教程手动抓Cookie后发送到此绑定。")
		}
	} else {
		cks = LimitJdCookie(cks, a)
		if len(cks) == 0 {
			sender.Reply("没有匹配的账号")
			return errors.New("没有匹配的账号")
		} else {
			for i := range cks {
				handle(&cks[i])
			}
		}
	}
	return nil
}

var codeSignals = []CodeSignal{
	{
		Command: []string{"status", "状态"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			return Count()
		},
	},
	{
		Command: []string{"打卡"},
		Handle: func(sender *Sender) interface{} {
			//if sender.Type == "tgg" {
			//	sender.Type = "tg"
			//}
			//if sender.Type == "qqg" {
			//	sender.Type = "qq"
			//}
			zero, _ := time.ParseInLocation("2006-01-02", time.Now().Local().Format("2006-01-02"), time.Local)
			var u User
			var ntime = time.Now()
			var first = false
			total := []int{}
			err := db.Where("number = ?", sender.UserID).First(&u).Error
			if err != nil {
				first = true
				u = User{
					Class:    sender.Type,
					Number:   sender.UserID,
					Coin:     1,
					ActiveAt: ntime,
				}
				if err := db.Create(&u).Error; err != nil {
					return err.Error()
				}
			} else {
				if zero.Unix() > u.ActiveAt.Unix() {
					first = true
				} else {
					return fmt.Sprintf("你打过卡了，余额%d。", u.Coin)
				}
			}
			if first {
				db.Model(User{}).Select("count(id) as total").Where("active_at > ?", zero).Pluck("total", &total)
				coin := 1
				if total[0]%3 == 0 {
					coin = 2
				}
				if total[0]%13 == 0 {
					coin = 8
				}
				db.Model(&u).Updates(map[string]interface{}{
					"active_at": ntime,
					"coin":      gorm.Expr(fmt.Sprintf("coin+%d", coin)),
				})
				u.Coin += coin
				sender.Reply(fmt.Sprintf("你是打卡第%d人，奖励%d个币，余额%d。", total[0]+1, coin, u.Coin))
				ReturnCoin(sender)
				return ""
			}
			return nil
		},
	},
	{
		Command: []string{"清零"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Priority, 1)
				
			})
			sender.Reply("优先级已清零")
			return nil
		},
	},
	{
		Command: []string{"更新优先级"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			coin:=GetCoin(sender.UserID)
			t:=time.Now()
			if t.Weekday().String()=="Monday"{
				sender.handleJdCookies(func(ck *JdCookie) {
					ck.Update(Priority, coin)
				})
				sender.Reply("优先级已更新")
				ClearCoin(sender.UserID)
			}else{
					sender.Reply("等周一再更新呆瓜")
				}
			return nil
		},
	},

	{
		Command: []string{"coin", "余额", "yu", "yue"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			return fmt.Sprintf("余额%d", GetCoin(sender.UserID))
		},
	},

	{
		Command: []string{"升级", "更新", "update", "upgrade"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			if err := Update(sender); err != nil {
				return err.Error()
			}
			sender.Reply("重启程序")
			Daemon()
			return nil
		},
	},
	{
		Command: []string{"重启", "reload", "restart", "reboot"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.Reply("重启程序")
			Daemon()
			return nil
		},
	},
	{
		Command: []string{"更新账号"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.Reply("更新所有账号")
			logs.Info("更新所有账号")
			updateCookie()
			return nil
		},
	},
	{
		Command: []string{"任务列表"},
		Admin:   true,
		Handle: func(_ *Sender) interface{} {
			rt := ""
			for i := range Config.Repos {
				for j := range Config.Repos[i].Task {
					rt += fmt.Sprintf("%s\t%s\n", Config.Repos[i].Task[j].Title, Config.Repos[i].Task[j].Cron)
				}
			}
			return rt
		},
	},
	{
		Command: []string{"查询2"},
		Handle: func(sender *Sender) interface{} {
		    
			sender.handleJdCookies(func(ck *JdCookie) {
				sender.Reply(ck.Query())
			})
			return nil
		},
	},
	{
		Command: []string{"查询"},
		Handle: func(sender *Sender) interface{} {
		    sender.Reply("开始查询，请稍候！此查询时间较久，请耐心等待，如长时间无响应，可以尝试回复【查询2】")
			sender.handleJdCookies(func(ck *JdCookie) {
				sender.Reply(ck.Query1())
			})
			return nil
		},
	},
	{
		Command: []string{"发送", "通知", "notify", "send"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			if len(sender.Contents) < 2 {
				sender.Reply("发送指令格式错误")
			} else {
				rt := strings.Join(sender.Contents[1:], " ")
				sender.Contents = sender.Contents[0:1]
				if sender.handleJdCookies(func(ck *JdCookie) {
					ck.Push(rt)
				}) == nil {
					return "操作成功"
				}
			}
			return nil
		},
	},
	{
		Command: []string{"设置管理员"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			ctt := sender.JoinContens()
			db.Create(&UserAdmin{Content: ctt})
			return "已设置管理员"
		},
	},
	{
		Command: []string{"取消管理员"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			ctt := sender.JoinContens()
			RemoveUserAdmin(ctt)
			return "已取消管理员"
		},
	},
	//{
	//	Command: []string{"按许愿币更新排名"},
	//	Admin:   true,
	//	Handle: func(sender *Sender) interface{} {
	//		cookies:= GetJdCookies()
	//		for i := range cookies {
	//			cookie := cookies[i]
	//			if cookie.QQ {
	//
	//			}
	//			cookie.Update(Priority,cookie.)
	//		}
	//		sender.handleJdCookies(func(ck *JdCookie) {
	//			sender.Reply(ck.Query())
	//		})
	//		return "已更新排行"
	//	},
	//},
	{
		Command: []string{"run", "执行", "运行"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			name := sender.Contents[0]
			pins := ""
			if len(sender.Contents) > 1 {
				sender.Contents = sender.Contents[1:]
				err := sender.handleJdCookies(func(ck *JdCookie) {
					pins += "&" + ck.PtPin
				})
				if err != nil {
					return nil
				}
			}
			envs := []Env{}
			if pins != "" {
				envs = append(envs, Env{
					Name:  "pins",
					Value: pins,
				})
			}
			runTask(&Task{Path: name, Envs: envs}, sender)
			return nil
		},
	},
	{
		Command: []string{"优先级", "priority"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			priority := Int(sender.Contents[0])
			if len(sender.Contents) > 1 {
				sender.Contents = sender.Contents[1:]
				sender.handleJdCookies(func(ck *JdCookie) {
					ck.Update(Priority, priority)
					sender.Reply(fmt.Sprintf("已设置账号%s(%s)的优先级为%d。", ck.PtPin, ck.Nickname, priority))
				})
			}
			return nil
		},
	},
	{
		Command: []string{"绑定QQ"},
		Handle: func(sender *Sender) interface{} {
			qq := Int(sender.Contents[0])
			if len(sender.Contents) > 1 {
				sender.Contents = sender.Contents[1:]
				sender.handleJdCookies(func(ck *JdCookie) {
					ck.Update(QQ, qq)
					sender.Reply(fmt.Sprintf("账号%s已绑定QQ为%v。", ck.Nickname, ck.QQ))
				})
			}
			return nil
		},
	},
	{
		Command: []string{"cmd", "command", "命令"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			ct := sender.JoinContens()
			if regexp.MustCompile(`rm\s+-rf`).FindString(ct) != "" {
				return "over"
			}
			cmd(ct, sender)
			return nil
		},
	},
	{
		Command: []string{"环境变量", "environments", "envs"},
		Admin:   true,
		Handle: func(_ *Sender) interface{} {
			rt := []string{}
			envs := GetEnvs()
			if len(envs) == 0 {
				return "未设置任何环境变量"
			}
			for _, env := range envs {
				rt = append(rt, fmt.Sprintf(`%s="%s"`, env.Name, env.Value))
			}
			return strings.Join(rt, "\n")
		},
	},
	{
		Command: []string{"get-env", "env", "e"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			ct := sender.JoinContens()
			if ct == "" {
				return "未指定变量名"
			}
			value := GetEnv(ct)
			if value == "" {
				return "未设置环境变量"
			}
			return fmt.Sprintf("环境变量的值为：" + value)
		},
	},
	{
		Command: []string{"set-env", "se", "export"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			env := &Env{}
			if len(sender.Contents) >= 2 {
				env.Name = sender.Contents[0]
				env.Value = strings.Join(sender.Contents[1:], " ")
			} else if len(sender.Contents) == 1 {
				ss := regexp.MustCompile(`^([^'"=]+)=['"]?([^=]+?)['"]?$`).FindStringSubmatch(sender.Contents[0])
				if len(ss) != 3 {
					return "无法解析"
				}
				env.Name = ss[1]
				env.Value = ss[2]
			} else {
				return "???"
			}
			ExportEnv(env)
			return "操作成功"
		},
	},
	{
		Command: []string{"unset-env", "ue", "unexport", "de"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			UnExportEnv(&Env{
				Name: sender.JoinContens(),
			})
			return "操作成功"
		},
	},
	{
		Command: []string{"reply", "回复"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			if len(sender.Contents) >= 2 {
				replies[sender.Contents[0]] = strings.Join(sender.Contents[1:], " ")
			} else {
				return "操作失败"
			}
			return "操作成功"
		},
	},
	{
		Command: []string{"help", "助力"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Help, True)
				sender.Reply(fmt.Sprintf("已设置助力账号%s(%s)", ck.PtPin, ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"tool", "工具人", "unhelp", "取消助力"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Help, False)
				sender.Reply(fmt.Sprintf("已设置取消助力账号%s(%s)", ck.PtPin, ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"屏蔽", "hack"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Priority, -1)
				sender.Reply(fmt.Sprintf("已屏蔽账号%s", ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"更新指定账号"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				if len(ck.WsKey) > 0 {
					var pinky = fmt.Sprintf("pin=%s;wskey=%s;", ck.PtPin, ck.WsKey)
					rsp := cmd(fmt.Sprintf(`python3 test.py "%s"`, pinky), &Sender{})
					if len(rsp) > 0 {
						ptKey := FetchJdCookieValue("pt_key", rsp)
						ptPin := FetchJdCookieValue("pt_pin", rsp)
						ck := JdCookie{
							PtKey: ptKey,
							PtPin: ptPin,
						}
						if nck, err := GetJdCookie(ck.PtPin); err == nil {
							nck.InPool(ck.PtKey)
							msg := fmt.Sprintf("更新账号成功，%s", ck.PtPin)
							sender.Reply(msg)
							logs.Info(msg)
						} else {
							sender.Reply("转换失败")
						}
					} else {
						sender.Reply(fmt.Sprintf("Wskey失效，%s", ck.Nickname))
					}
				} else {
					sender.Reply(fmt.Sprintf("Wskey为空，%s", ck.Nickname))
				}

			})
			return nil
		},
	},
	{
		Command: []string{"删除", "clean"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Removes(ck)
				sender.Reply(fmt.Sprintf("已删除账号%s", ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"清理过期账号"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.Reply(fmt.Sprintf("删除所有false账号，请慎用"))
			sender.handleJdCookies(func(ck *JdCookie) {
				if ck.Available == False {
					ck.Removes(ck)
					sender.Reply(fmt.Sprintf("已清理账号%s", ck.Nickname))
				}
			})
			return nil
		},
	},
	{
		Command: []string{"取消屏蔽", "unhack"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				ck.Update(Priority, 2)
				sender.Reply(fmt.Sprintf("已取消屏蔽账号%s", ck.Nickname))
			})
			return nil
		},
	},
	{
		Command: []string{"转账"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			cost := 1
			if sender.ReplySenderUserID == 0 {
				return "没有转账目标。"
			}
			amount := Int(sender.JoinContens())
			if !sender.IsAdmin {
				if amount <= 0 {
					return "转账金额必须大于等于1。"
				}
			}
			if sender.UserID == sender.ReplySenderUserID {
				db.Model(User{}).Where("number = ?", sender.UserID).Updates(map[string]interface{}{
					"coin": gorm.Expr(fmt.Sprintf("coin - %d", cost)),
				})
				return fmt.Sprintf("转账成功，扣除手续费%d枚东币。", cost)
			}
			if amount > 10000 {
				return "单笔转账限额10000。"
			}
			tx := db.Begin()
			s := &User{}
			if err := db.Where("number = ?", sender.UserID).First(&s).Error; err != nil {
				tx.Rollback()
				return "你还没有开通钱包功能。"
			}
			if s.Coin < amount {
				tx.Rollback()
				return "余额不足。"
			}
			real := amount
			if !sender.IsAdmin {
				if amount <= cost {
					tx.Rollback()
					return fmt.Sprintf("转账失败，手续费需要%d个东币。", cost)
				}
				real = amount - cost
			} else {
				cost = 0
			}
			r := &User{}
			if err := db.Where("number = ?", sender.ReplySenderUserID).First(&r).Error; err != nil {
				tx.Rollback()
				return "他还没有开通钱包功能"
			}
			if tx.Model(User{}).Where("number = ?", sender.UserID).Updates(map[string]interface{}{
				"coin": gorm.Expr(fmt.Sprintf("coin - %d", amount)),
			}).RowsAffected == 0 {
				tx.Rollback()
				return "转账失败"
			}
			if tx.Model(User{}).Where("number = ?", sender.ReplySenderUserID).Updates(map[string]interface{}{
				"coin": gorm.Expr(fmt.Sprintf("coin + %d", real)),
			}).RowsAffected == 0 {
				tx.Rollback()
				return "转账失败"
			}
			tx.Commit()
			return fmt.Sprintf("转账成功，你的余额%d，他的余额%d，手续费%d。", s.Coin-amount, r.Coin+real, cost)
		},
	},
	{
		Command: []string{"导出"},
		Admin:   true,
		Handle: func(sender *Sender) interface{} {
			sender.handleJdCookies(func(ck *JdCookie) {
				sender.Reply(fmt.Sprintf("pt_key=%s;pt_pin=%s;", ck.PtKey, ck.PtPin))
			})
			return nil
		},
	},
}

var mx = map[int]bool{}

func LimitJdCookie(cks []JdCookie, a string) []JdCookie {
	ncks := []JdCookie{}
	if s := strings.Split(a, "-"); len(s) == 2 {
		for i := range cks {
			if i+1 >= Int(s[0]) && i+1 <= Int(s[1]) {
				ncks = append(ncks, cks[i])
			}
		}
	} else if x := regexp.MustCompile(`^[\s\d,]+$`).FindString(a); x != "" {
		xx := regexp.MustCompile(`(\d+)`).FindAllStringSubmatch(a, -1)
		for i := range cks {
			for _, x := range xx {
				if fmt.Sprint(i+1) == x[1] {
					ncks = append(ncks, cks[i])
				}
			}

		}
	} else if a != "" {
		a = strings.Replace(a, " ", "", -1)
		for i := range cks {
			if strings.Contains(cks[i].Note, a) || strings.Contains(cks[i].Nickname, a) || strings.Contains(cks[i].PtPin, a) {
				ncks = append(ncks, cks[i])
			}
		}
	}
	return ncks
}

func ReturnCoin(sender *Sender) {
	tx := db.Begin()
	ws := []Wish{}
	if err := tx.Where("status = 0 and user_number = ?", sender.UserID).Find(&ws).Error; err != nil {
		tx.Rollback()
		sender.Reply(err.Error())
	}
	for _, w := range ws {
		if tx.Model(User{}).Where("number = ? ", sender.UserID).Update(
			"coin", gorm.Expr(fmt.Sprintf("coin + %d", w.Coin)),
		).RowsAffected == 0 {
			tx.Rollback()
			sender.Reply("愿望未达成退还东币失败。")
			return
		}
		sender.Reply(fmt.Sprintf("愿望未达成退还%d枚东币。", w.Coin))
		if tx.Model(&w).Update(
			"status", 1,
		).RowsAffected == 0 {
			tx.Rollback()
			sender.Reply("愿望未达成退还东币失败。")
			return
		}
	}
	tx.Commit()
}
