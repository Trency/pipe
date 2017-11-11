// Pipe - A small and beautiful blogging platform written in golang.
// Copyright (C) 2017, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package controller

import (
	"net/http"
	"time"

	"github.com/b3log/pipe/model"
	"github.com/b3log/pipe/service"
	"github.com/b3log/pipe/util"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
)

type DataModel map[string]interface{}

const nilB3id = "H9oxzSym"

func fillUser(c *gin.Context) {
	inited := service.Init.Inited()
	if !inited && util.PathInit != c.Request.URL.Path {
		c.Redirect(http.StatusSeeOther, util.Conf.Server+util.PathInit)
		c.Abort()

		return
	}

	dataModel := &DataModel{}
	c.Set("dataModel", dataModel)
	session := util.GetSession(c)
	(*dataModel)["User"] = session
	if 0 != session.UID {
		c.Next()

		return
	}

	b3id := c.Request.URL.Query().Get("b3id")
	switch b3id {
	case nilB3id:
		c.Next()

		return
	case "":
		redirectURL := util.Conf.Server + c.Request.URL.Path
		c.Redirect(http.StatusSeeOther, util.HacPaiURL+"/apis/b3-identity?goto="+redirectURL)
		c.Abort()

		return
	default:
		result := util.NewResult()
		request := gorequest.New()
		_, _, errs := request.Get(util.HacPaiURL+"/apis/check-b3-identity?b3id="+b3id).
			Set("user-agent", util.UserAgent).Timeout(30 * time.Second).EndStruct(result)
		if nil != errs {
			logger.Errorf("check b3 identity failed: %s", errs)
			c.Next()

			return
		}

		if 0 != result.Code {
			c.Next()

			return
		}

		processB3IDResult(result)
		data := result.Data.(map[string]interface{})

		username := data["userName"].(string)
		b3Key := data["userB3Key"].(string)
		userAvatar := data["userAvatarURL"].(string)

		session = &util.SessionData{
			UName:   username,
			UB3Key:  b3Key,
			UAvatar: userAvatar,
			URole:   model.UserRoleBlogAdmin,
		}

		user := &model.User{
			Name:      session.UName,
			B3Key:     b3Key,
			AvatarURL: session.UAvatar,
		}

		if service.Init.Inited() {
			if err := service.Init.InitBlog(user); nil != err {
				logger.Errorf("init user [name=%s] blog failed: %s", username, err.Error())
			}
		}

		if existUser := service.User.GetUserByName(username); nil != existUser {
			session.UAvatar = existUser.AvatarURL
			ownBlog := service.User.GetOwnBlog(existUser.ID)
			if nil != ownBlog {
				session.BID = ownBlog.ID
				session.BURL = ownBlog.URL
				session.URole = ownBlog.UserRole
			}
			session.UID = existUser.ID
		} else {
			if err := service.User.AddUser(user); nil != err {
				logger.Errorf("add user [name=%s] failed: %s", username, err.Error())
			}

			session.UID = user.ID
		}

		if err := session.Save(c); nil != err {
			result.Code = -1
			result.Msg = "saves session failed: " + err.Error()
		}

		(*dataModel)["User"] = session
		c.Next()
	}
}
